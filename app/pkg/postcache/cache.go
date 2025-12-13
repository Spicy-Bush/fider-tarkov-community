package postcache

import (
	"sync"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

// if we ever run multiple instances, each
// will have to have its own cache. This is
// probably fine for the current use case,
// but worth noting as reference for future

type CachedRanking struct {
	PostIDs   []int
	CreatedAt time.Time
}

type TenantCache struct {
	rankings       sync.Map
	tags           []*entity.Tag
	tagsMu         sync.RWMutex
	countPerStatus map[enum.PostStatus]int
	countMu        sync.RWMutex
}

var (
	tenantCaches sync.Map
	cacheTTL     = 60 * time.Second
)

func getTenantCache(tenantID int) *TenantCache {
	if tc, ok := tenantCaches.Load(tenantID); ok {
		return tc.(*TenantCache)
	}

	tc := &TenantCache{}
	actual, _ := tenantCaches.LoadOrStore(tenantID, tc)
	return actual.(*TenantCache)
}

func GetRanking(tenantID int, viewType string) ([]int, bool) {
	tc := getTenantCache(tenantID)

	if val, ok := tc.rankings.Load(viewType); ok {
		ranking := val.(*CachedRanking)
		if time.Since(ranking.CreatedAt) < cacheTTL {
			return ranking.PostIDs, true
		}
		tc.rankings.Delete(viewType)
	}
	return nil, false
}

func SetRanking(tenantID int, viewType string, postIDs []int) {
	tc := getTenantCache(tenantID)

	tc.rankings.Store(viewType, &CachedRanking{
		PostIDs:   postIDs,
		CreatedAt: time.Now(),
	})
}

func InvalidateTenantRankings(tenantID int) {
	tc := getTenantCache(tenantID)

	tc.rankings.Range(func(key, value interface{}) bool {
		tc.rankings.Delete(key)
		return true
	})
}

func IsCacheable(view string, myVotesOnly bool, myPostsOnly bool, notMyVotes bool, tags []string, query string, untagged bool) bool {
	if myVotesOnly || myPostsOnly || notMyVotes || untagged {
		return false
	}
	if len(tags) > 0 {
		return false
	}
	if query != "" {
		return false
	}

	switch view {
	case "trending", "newest", "most-wanted", "most-discussed", "controversial", "recently-updated", "all":
		return true
	default:
		return false
	}
}

func GetCacheKey(view string) string {
	return view
}

func GetTags(tenantID int) ([]*entity.Tag, bool) {
	tc := getTenantCache(tenantID)
	tc.tagsMu.RLock()
	defer tc.tagsMu.RUnlock()
	if tc.tags != nil {
		return tc.tags, true
	}
	return nil, false
}

func SetTags(tenantID int, tags []*entity.Tag) {
	tc := getTenantCache(tenantID)
	tc.tagsMu.Lock()
	tc.tags = tags
	tc.tagsMu.Unlock()
}

func InvalidateTags(tenantID int) {
	tc := getTenantCache(tenantID)
	tc.tagsMu.Lock()
	tc.tags = nil
	tc.tagsMu.Unlock()
}

func GetCountPerStatus(tenantID int) (map[enum.PostStatus]int, bool) {
	tc := getTenantCache(tenantID)
	tc.countMu.RLock()
	defer tc.countMu.RUnlock()
	if tc.countPerStatus != nil {
		return tc.countPerStatus, true
	}
	return nil, false
}

func SetCountPerStatus(tenantID int, counts map[enum.PostStatus]int) {
	tc := getTenantCache(tenantID)
	tc.countMu.Lock()
	tc.countPerStatus = counts
	tc.countMu.Unlock()
}

func InvalidateCountPerStatus(tenantID int) {
	tc := getTenantCache(tenantID)
	tc.countMu.Lock()
	tc.countPerStatus = nil
	tc.countMu.Unlock()
}
