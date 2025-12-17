package pages

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
)

type CachedEmbeddedData struct {
	Posts    []*entity.Post `json:"posts,omitempty"`
	CachedAt time.Time      `json:"cachedAt"`
	PostIDs  []int          `json:"postIds,omitempty"`
}

func RefreshEmbeddedData(ctx context.Context, content string) (*CachedEmbeddedData, error) {
	embeddedContent := markdown.ParseEmbeddedTables(content)
	if len(embeddedContent) == 0 {
		return nil, nil
	}

	allPosts := make([]*entity.Post, 0)
	allPostIDs := make([]int, 0)

	for _, embedded := range embeddedContent {
		var posts []*entity.Post
		var err error

		switch embedded.Type {
		case "posts":
			if len(embedded.PostIDs) > 0 {
				posts, err = fetchPostsByIDs(ctx, embedded.PostIDs)
				if err == nil && embedded.Limit > 0 && len(posts) > embedded.Limit {
					posts = posts[:embedded.Limit]
				}
			} else {
				posts, err = fetchPostsByFilters(ctx, embedded.Filters, embedded.Limit)
			}
		case "post":
			if len(embedded.PostIDs) > 0 {
				posts, err = fetchPostsByIDs(ctx, embedded.PostIDs)
			}
		}

		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch embedded posts")
		}

		allPosts = append(allPosts, posts...)
		for _, post := range posts {
			allPostIDs = append(allPostIDs, post.ID)
		}
	}

	if len(allPosts) == 0 {
		return nil, nil
	}

	return &CachedEmbeddedData{
		Posts:    allPosts,
		CachedAt: time.Now(),
		PostIDs:  allPostIDs,
	}, nil
}

func fetchPostsByFilters(ctx context.Context, filters map[string]string, limit int) ([]*entity.Post, error) {
	if limit <= 0 {
		limit = 100
	}

	q := &query.SearchPosts{
		Limit: strconv.Itoa(limit),
		Statuses: []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
		},
	}

	for key, value := range filters {
		switch key {
		case "tag":
			q.Tags = append(q.Tags, strings.ToLower(value))
		case "status":
			var status enum.PostStatus
			if err := status.UnmarshalText([]byte(value)); err == nil {
				q.Statuses = []enum.PostStatus{status}
			}
		case "query":
			q.Query = value
		}
	}

	if err := bus.Dispatch(ctx, q); err != nil {
		return nil, err
	}

	return q.Result, nil
}

func MarshalCachedData(data *CachedEmbeddedData) (string, error) {
	if data == nil {
		return "", nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal cached data")
	}
	return string(bytes), nil
}

func UnmarshalCachedData(data string) (*CachedEmbeddedData, error) {
	if data == "" {
		return nil, nil
	}
	var cached CachedEmbeddedData
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal cached data")
	}
	return &cached, nil
}

func fetchPostsByIDs(ctx context.Context, postIDs []int) ([]*entity.Post, error) {
	q := &query.GetPostsByIDs{
		PostIDs: postIDs,
	}

	if err := bus.Dispatch(ctx, q); err != nil {
		return nil, err
	}

	return q.Result, nil
}
