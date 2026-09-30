package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"sync"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

const (
	maxDiscussionTreeComments = 100_000
	// Limit retained rank payload to 12 MiB on 64-bit servers while keeping many small discussions.
	maxCachedDiscussionRanks  = 262_144
	maxCachedDiscussionOwners = 1_024
)

type discussionTreeKey struct {
	TenantID  int
	Kind      string
	OwnerID   int
	Concealed [32]byte
}

type discussionReplyRank struct {
	ID        int
	ParentID  int
	Score     int64
	CreatedAt time.Time
}

type discussionTree struct {
	Generation int64
	Ranks      []discussionReplyRank
	Oversized  bool
	LastUsed   uint64
}

type discussionReplySelection struct {
	IDs      []int
	Scores   []int64
	Uncached bool
}

type discussionTreeCache struct {
	sync.Mutex
	entries   map[discussionTreeKey]discussionTree
	rankCount int
	lastUsed  uint64
	maxRanks  int
	maxOwners int
	builds    chan struct{}
}

var discussionTrees = discussionTreeCache{
	entries:   make(map[discussionTreeKey]discussionTree),
	maxRanks:  maxCachedDiscussionRanks,
	maxOwners: maxCachedDiscussionOwners,
	builds:    make(chan struct{}, 2),
}

func (cache *discussionTreeCache) lookup(key discussionTreeKey, generation int64, q *query.GetDiscussionComments) (discussionReplySelection, bool) {
	cache.Lock()
	defer cache.Unlock()

	tree, found := cache.entries[key]
	if !found || tree.Generation != generation {
		return discussionReplySelection{}, false
	}

	cache.lastUsed++
	tree.LastUsed = cache.lastUsed
	cache.entries[key] = tree

	return selectDiscussionReplies(tree, q), true
}

func (cache *discussionTreeCache) store(key discussionTreeKey, tree discussionTree) {
	cache.Lock()
	defer cache.Unlock()

	if previous, exists := cache.entries[key]; exists {
		cache.rankCount -= len(previous.Ranks)
	}

	cache.lastUsed++
	tree.LastUsed = cache.lastUsed
	cache.entries[key] = tree
	cache.rankCount += len(tree.Ranks)

	for len(cache.entries) > cache.maxOwners || cache.rankCount > cache.maxRanks {
		var oldest discussionTreeKey
		order := cache.lastUsed
		for candidate, entry := range cache.entries {
			if entry.LastUsed <= order {
				oldest, order = candidate, entry.LastUsed
			}
		}

		cache.rankCount -= len(cache.entries[oldest].Ranks)
		delete(cache.entries, oldest)
	}
}

func selectDiscussionReplies(tree discussionTree, q *query.GetDiscussionComments) discussionReplySelection {
	selection := discussionReplySelection{Uncached: tree.Oversized}
	if tree.Oversized {
		return selection
	}

	parentID := 0
	if q.ParentID != nil {
		parentID = *q.ParentID
	}

	start := sort.Search(len(tree.Ranks), func(index int) bool {
		return tree.Ranks[index].ParentID >= parentID
	})
	end := sort.Search(len(tree.Ranks), func(index int) bool {
		return tree.Ranks[index].ParentID > parentID
	})

	siblings := tree.Ranks[start:end]
	if q.AfterID != 0 {
		start = sort.Search(len(siblings), func(index int) bool {
			comment := siblings[index]
			if comment.Score != int64(q.AfterScore) {
				return comment.Score < int64(q.AfterScore)
			}

			if !comment.CreatedAt.Equal(q.After) {
				return comment.CreatedAt.Before(q.After)
			}

			return comment.ID < q.AfterID
		})
		siblings = siblings[start:]
	}

	if len(siblings) > 26 {
		siblings = siblings[:26]
	}

	selection.IDs = make([]int, len(siblings))
	selection.Scores = make([]int64, len(siblings))
	for index, comment := range siblings {
		selection.IDs[index] = comment.ID
		selection.Scores[index] = comment.Score
	}

	return selection
}

func prepareDiscussionReplies(ctx context.Context, trx *dbx.Trx, q *query.GetDiscussionComments, ownerColumn string, tenantID int) (discussionReplySelection, error) {
	key := discussionTreeKey{TenantID: tenantID, Kind: q.Discussion.Owner.Kind, OwnerID: q.Discussion.Owner.ID}
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	args := []any{tenantID, viewerID(user), key.OwnerID}
	source := discussionSource(ctx, ownerColumn, &args)
	rows, err := trx.Query(source+`
        SELECT id, COALESCE(parent_id, 0) FROM discussion_comments
        WHERE (deleted_at IS NOT NULL OR moderation_pending) AND NOT visible
        ORDER BY id
    `, args...)
	if err != nil {
		return discussionReplySelection{}, err
	}

	concealed := make(map[int]int)
	fingerprint := sha256.New()
	var encodedID [8]byte
	for rows.Next() {
		var id, parentID int
		if err := rows.Scan(&id, &parentID); err != nil {
			rows.Close()
			return discussionReplySelection{}, err
		}

		if ancestor, hidden := concealed[parentID]; hidden {
			parentID = ancestor
		}
		concealed[id] = parentID
		binary.LittleEndian.PutUint64(encodedID[:], uint64(id))
		fingerprint.Write(encodedID[:])
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return discussionReplySelection{}, err
	}
	if err := rows.Close(); err != nil {
		return discussionReplySelection{}, err
	}
	if len(concealed) > 0 {
		copy(key.Concealed[:], fingerprint.Sum(nil))
	}

	versionSQL := `SELECT COALESCE((SELECT generation FROM discussion_tree_versions
        WHERE tenant_id = $1 AND ` + pq.QuoteIdentifier(ownerColumn) + ` = $2), 0)`

	var generation int64
	if err := trx.Scalar(&generation, versionSQL, tenantID, key.OwnerID); err != nil {
		return discussionReplySelection{}, err
	}

	selection, found := discussionTrees.lookup(key, generation, q)
	if found && !selection.Uncached {
		return selection, nil
	}

	select {
	case discussionTrees.builds <- struct{}{}:
		defer func() { <-discussionTrees.builds }()
	case <-ctx.Done():
		return discussionReplySelection{}, ctx.Err()
	}

	selection, found = discussionTrees.lookup(key, generation, q)
	if found && !selection.Uncached {
		return selection, nil
	}

	if found {
		parentID := 0
		if q.ParentID != nil {
			parentID = *q.ParentID
		}
		tree, err := readDiscussionTree(trx, key, ownerColumn, 0, concealed, &parentID)
		return selectDiscussionReplies(tree, q), err
	}

	tree, err := readDiscussionTree(trx, key, ownerColumn, maxDiscussionTreeComments, concealed, nil)
	if err != nil {
		return discussionReplySelection{}, err
	}

	tree.Generation = generation
	var currentGeneration int64
	if err := trx.Scalar(&currentGeneration, versionSQL, tenantID, key.OwnerID); err != nil {
		return discussionReplySelection{}, err
	}

	if generation != 0 && currentGeneration == generation {
		discussionTrees.store(key, tree)
	}

	if tree.Oversized {
		parentID := 0
		if q.ParentID != nil {
			parentID = *q.ParentID
		}
		tree, err = readDiscussionTree(trx, key, ownerColumn, 0, concealed, &parentID)
		if err != nil {
			return discussionReplySelection{}, err
		}
	}

	return selectDiscussionReplies(tree, q), nil
}

func readDiscussionTree(trx *dbx.Trx, key discussionTreeKey, ownerColumn string, limit int, concealed map[int]int, siblingsOf *int) (discussionTree, error) {
	var rowLimit *int
	if limit > 0 {
		maximum := limit + 1
		rowLimit = &maximum
	}

	rows, err := trx.Query(`
        SELECT id, COALESCE(parent_id, 0), created_at
        FROM comments WHERE tenant_id = $1 AND `+pq.QuoteIdentifier(ownerColumn)+` = $2
        ORDER BY id DESC LIMIT $3
    `, key.TenantID, key.OwnerID, rowLimit)
	if err != nil {
		return discussionTree{}, err
	}

	defer rows.Close()

	descendants := make(map[int]int64)
	ranks := make([]discussionReplyRank, 0, 256)
	var rawID, rawParentID int64
	var createdAt time.Time
	counted := 0
	for rows.Next() {
		if err := rows.Scan(&rawID, &rawParentID, &createdAt); err != nil {
			return discussionTree{}, err
		}

		counted++
		if limit > 0 && counted > limit {
			return discussionTree{Oversized: true}, rows.Close()
		}

		id, parentID := int(rawID), int(rawParentID)
		count := descendants[id]
		delete(descendants, id)
		_, hidden := concealed[id]
		if !hidden {
			visibleParent := parentID
			if ancestor, hiddenParent := concealed[parentID]; hiddenParent {
				visibleParent = ancestor
			}
			if siblingsOf == nil || visibleParent == *siblingsOf {
				ranks = append(ranks, discussionReplyRank{ID: id, ParentID: visibleParent, Score: count, CreatedAt: createdAt})
			}
		}

		if parentID != 0 {
			if !hidden {
				count++
			}

			descendants[parentID] += count
		}
	}

	if err := rows.Err(); err != nil {
		return discussionTree{}, err
	}

	if err := rows.Close(); err != nil {
		return discussionTree{}, err
	}

	sort.Slice(ranks, func(left, right int) bool {
		if ranks[left].ParentID != ranks[right].ParentID {
			return ranks[left].ParentID < ranks[right].ParentID
		}

		if ranks[left].Score != ranks[right].Score {
			return ranks[left].Score > ranks[right].Score
		}

		if !ranks[left].CreatedAt.Equal(ranks[right].CreatedAt) {
			return ranks[left].CreatedAt.After(ranks[right].CreatedAt)
		}

		return ranks[left].ID > ranks[right].ID
	})

	prepared := make([]discussionReplyRank, len(ranks))
	copy(prepared, ranks)
	return discussionTree{Ranks: prepared}, nil
}
