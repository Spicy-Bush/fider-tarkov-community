package handlers

import (
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func ArchivePostsPage() web.HandlerFunc {
	return func(c *web.Context) error {
		getAllTags := &query.GetAllTags{}
		if err := bus.Dispatch(c, getAllTags); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageArchive.page",
			Title: "Archive Posts - Site Settings",
			Data: web.Map{
				"tags": getAllTags.Result,
			},
		})
	}
}

func ListArchivablePosts() web.HandlerFunc {
	return func(c *web.Context) error {
		page, _ := c.QueryParamAsInt("page")
		perPage, _ := c.QueryParamAsInt("perPage")

		const maxInt32 = 2147483647
		if page > maxInt32 {
			page = maxInt32
		}
		if perPage > maxInt32 {
			perPage = maxInt32
		}

		q := &query.GetArchivablePosts{
			Page:    page,
			PerPage: perPage,
		}

		if createdBefore := c.QueryParam("createdBefore"); createdBefore != "" {
			t, err := time.Parse(time.RFC3339, createdBefore)
			if err == nil {
				q.CreatedBefore = &t
			}
		}

		if inactiveSince := c.QueryParam("inactiveSince"); inactiveSince != "" {
			t, err := time.Parse(time.RFC3339, inactiveSince)
			if err == nil {
				q.InactiveSince = &t
			}
		}

		if maxVotes, err := c.QueryParamAsInt("maxVotes"); err == nil && maxVotes > 0 {
			if maxVotes > maxInt32 {
				maxVotes = maxInt32
			}
			q.MaxVotes = &maxVotes
		}

		if maxComments, err := c.QueryParamAsInt("maxComments"); err == nil && maxComments > 0 {
			if maxComments > maxInt32 {
				maxComments = maxInt32
			}
			q.MaxComments = &maxComments
		}

		if statuses := c.QueryParamAsArray("statuses"); len(statuses) > 0 {
			for _, s := range statuses {
				var status enum.PostStatus
				if err := status.UnmarshalText([]byte(s)); err == nil {
					q.Statuses = append(q.Statuses, status)
				}
			}
		}

		if tags := c.QueryParamAsArray("tags"); len(tags) > 0 {
			q.Tags = tags
		}

		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"posts": q.Result,
			"total": q.Total,
		})
	}
}

func ArchivePost() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		if getPost.Result == nil {
			return c.NotFound()
		}

		if getPost.Result.Status == enum.PostDeleted || getPost.Result.Status == enum.PostArchived {
			return c.BadRequest(web.Map{"message": "This post cannot be archived"})
		}

		archiveCmd := &cmd.ArchivePost{Post: getPost.Result}
		if err := bus.Dispatch(c, archiveCmd); err != nil {
			return c.Failure(err)
		}

		postcache.InvalidateTenantRankings(c.Tenant().ID)
		postcache.InvalidateCountPerStatus(c.Tenant().ID)

		return c.Ok(web.Map{})
	}
}

func UnarchivePost() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		if getPost.Result == nil {
			return c.NotFound()
		}

		if getPost.Result.Status != enum.PostArchived {
			return c.BadRequest(web.Map{"message": "This post is not archived"})
		}

		unarchiveCmd := &cmd.UnarchivePost{Post: getPost.Result, Reason: "Manual unarchive"}
		if err := bus.Dispatch(c, unarchiveCmd); err != nil {
			return c.Failure(err)
		}

		postcache.InvalidateTenantRankings(c.Tenant().ID)
		postcache.InvalidateCountPerStatus(c.Tenant().ID)

		return c.Ok(web.Map{})
	}
}

type bulkArchiveInput struct {
	PostIDs []int `json:"postIds"`
}

func BulkArchive() web.HandlerFunc {
	return func(c *web.Context) error {
		input := new(bulkArchiveInput)
		if err := c.Bind(input); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid request"})
		}

		if len(input.PostIDs) == 0 {
			return c.BadRequest(web.Map{"message": "No posts selected"})
		}

		bulkCmd := &cmd.BulkArchivePosts{PostIDs: input.PostIDs}
		if err := bus.Dispatch(c, bulkCmd); err != nil {
			return c.Failure(err)
		}

		postcache.InvalidateTenantRankings(c.Tenant().ID)
		postcache.InvalidateCountPerStatus(c.Tenant().ID)

		return c.Ok(web.Map{"archived": len(input.PostIDs)})
	}
}

