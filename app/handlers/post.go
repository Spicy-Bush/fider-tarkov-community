package handlers

import (
	"fmt"
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/csv"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// Index is the default home page
func Index() web.HandlerFunc {
	return func(c *web.Context) error {
		c.SetCanonicalURL("")

		tenantID := c.Tenant().ID

		var (
			tags           []*entity.Tag
			countPerStatus map[enum.PostStatus]int
		)

		if cached, ok := postcache.GetTags(tenantID); ok {
			tags = cached
		} else {
			q := &query.GetAllTags{}
			if err := bus.Dispatch(c, q); err != nil {
				return c.Failure(err)
			}
			tags = q.Result
			postcache.SetTags(tenantID, tags)
		}

		if cached, ok := postcache.GetCountPerStatus(tenantID); ok {
			countPerStatus = cached
		} else {
			q := &query.CountPostPerStatus{}
			if err := bus.Dispatch(c, q); err != nil {
				return c.Failure(err)
			}
			countPerStatus = q.Result
			postcache.SetCountPerStatus(tenantID, countPerStatus)
		}

		q := c.Request.URL.Query()
		view := q.Get("view")
		if view == "" {
			view = "trending"
		}

		tagLogic := q.Get("taglogic")
		if tagLogic == "" {
			tagLogic = "OR"
		}

		selectedTags := append([]string{}, q["tags"]...)
		searchPosts := &query.SearchPosts{
			Query:       q.Get("query"),
			View:        view,
			Limit:       "20",
			Tags:        selectedTags,
			Statuses:    []enum.PostStatus{},
			Date:        q.Get("date"),
			TagLogic:    tagLogic,
			MyVotesOnly: q.Get("myvotes") == "true",
			MyPostsOnly: q.Get("myposts") == "true",
			NotMyVotes:  q.Get("notmyvotes") == "true" || (len(q) == 0 && c.IsAuthenticated()),
		}
		searchPosts.SetStatusesFromStrings(q["statuses"])

		for _, tag := range selectedTags {
			if tag == "untagged" {
				searchPosts.Untagged = true
				searchPosts.Tags = nil
				break
			}
		}

		if err := bus.Dispatch(c, searchPosts); err != nil {
			return c.Failure(err)
		}

		description := ""
		if c.Tenant().WelcomeMessage != "" {
			description = markdown.PlainText(c.Tenant().WelcomeMessage)
		} else {
			description = "We'd love to hear what you're thinking about. What can we do better? This is the place for you to vote, discuss and share posts."
		}

		return c.Page(http.StatusOK, web.Props{
			Page:        "Home/Home.page",
			Description: description,
			Data: web.Map{
				"posts":          searchPosts.Result,
				"tags":           tags,
				"countPerStatus": countPerStatus,
				"initialFilters": web.Map{
					"query":      searchPosts.Query,
					"view":       searchPosts.View,
					"limit":      20,
					"tags":       selectedTags,
					"statuses":   searchPosts.Statuses,
					"date":       searchPosts.Date,
					"tagLogic":   searchPosts.TagLogic,
					"myVotes":    searchPosts.MyVotesOnly,
					"myPosts":    searchPosts.MyPostsOnly,
					"notMyVotes": searchPosts.NotMyVotes,
				},
			},
		})
	}
}

// PostDetails shows details of given Post by id
func PostDetails() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		c.SetCanonicalURL(fmt.Sprintf("/posts/%d/%s", getPost.Result.Number, getPost.Result.Slug))

		isSubscribed := &query.UserSubscribedTo{PostID: getPost.Result.ID}
		getAllTags := &query.GetAllTags{}
		getAttachments := &query.GetPostAttachments{PostID: getPost.Result.ID}
		getReportReasons := &query.GetReportReasons{}
		if err := bus.Dispatch(c, getAllTags, isSubscribed, getAttachments, getReportReasons); err != nil {
			return c.Failure(err)
		}

		// Get votes for avatar display
		listVotes := &query.ListPostVotes{PostID: getPost.Result.ID, Limit: 8, IncludeEmail: false}
		if err := bus.Dispatch(c, listVotes); err != nil {
			return c.Failure(err)
		}

		// For non-staff users, strip out the VoteType to anonymise what each person voted
		isStaff := c.User() != nil && (c.User().IsCollaborator() || c.User().IsModerator() || c.User().IsAdministrator())
		votes := listVotes.Result
		if !isStaff {
			// Create anonymous votes without VoteType for regular users
			votes = make([]*entity.Vote, len(listVotes.Result))
			for i, v := range listVotes.Result {
				votes[i] = &entity.Vote{
					User:      v.User,
					CreatedAt: v.CreatedAt,
					// VoteType intentionally omitted (will be zero value)
				}
			}
		}

		data := web.Map{
			"subscribed":    isSubscribed.Result,
			"post":          getPost.Result,
			"tags":          getAllTags.Result,
			"votes":         votes,
			"attachments":   getAttachments.Result,
			"reportReasons": getReportReasons.Result,
		}

		if c.User() != nil {
			reportedItems := &query.GetUserReportStatus{
				PostID: getPost.Result.ID,
			}

			if err := bus.Dispatch(c, reportedItems); err != nil {
				return c.Failure(err)
			}

			data["reportStatus"] = web.Map{
				"hasReportedPost":    reportedItems.HasReportedPost,
				"dailyLimitReached":  reportedItems.CountToday >= c.Tenant().DailyReportLimit(),
			}
		}

		return c.Page(http.StatusOK, web.Props{
			Page:        "ShowPost/ShowPost.page",
			Title:       getPost.Result.Title,
			Description: markdown.PlainText(getPost.Result.Description),
			Data:        data,
		})
	}
}

// ExportPostsToCSV returns a CSV with all posts
func ExportPostsToCSV() web.HandlerFunc {
	return func(c *web.Context) error {

		allPosts := &query.GetAllPosts{}
		if err := bus.Dispatch(c, allPosts); err != nil {
			return c.Failure(err)
		}

		bytes, err := csv.FromPosts(allPosts.Result)
		if err != nil {
			return c.Failure(err)
		}

		return c.Attachment("posts.csv", "text/csv", bytes)
	}
}
