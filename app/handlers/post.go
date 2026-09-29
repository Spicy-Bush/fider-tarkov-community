package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/csv"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

type savedPostFilters struct {
	TagIDs           []int    `json:"tagIds"`
	Statuses         []string `json:"statuses"`
	View             string   `json:"view"`
	Date             string   `json:"date"`
	TagLogic         string   `json:"tagLogic"`
	MyVotes          bool     `json:"myVotes"`
	MyPosts          bool     `json:"myPosts"`
	NotMyVotes       bool     `json:"notMyVotes"`
	Limit            int      `json:"limit"`
	Timestamp        int64    `json:"timestamp"`
	WasAuthenticated bool     `json:"wasAuthenticated"`
}

// Index is the default home page
func Index() web.HandlerFunc {
	return func(c *web.Context) error {
		c.SetCanonicalURL("")

		tags := &query.GetAllTags{}
		if err := bus.Dispatch(c, tags); err != nil {
			return c.Failure(err)
		}

		counts := &query.CountPostPerStatus{}
		if err := bus.Dispatch(c, counts); err != nil {
			return c.Failure(err)
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
		limit := 15
		var savedFiltersAt int64

		if len(q) == 0 {
			if cookie, err := c.Request.Cookie("pfilter"); err == nil && len(cookie.Value) <= 4096 {
				value, decodeErr := url.QueryUnescape(cookie.Value)
				var saved savedPostFilters
				if decodeErr == nil && json.Unmarshal([]byte(value), &saved) == nil && saved.Timestamp > 0 {
					savedFiltersAt = saved.Timestamp
					slugs := map[int]string{0: "untagged"}
					for _, tag := range tags.Result {
						slugs[tag.ID] = tag.Slug
					}

					for _, id := range saved.TagIDs {
						if slug, found := slugs[id]; found {
							selectedTags = append(selectedTags, slug)
						}
					}
					searchPosts.Tags = selectedTags
					searchPosts.SetStatusesFromStrings(saved.Statuses)
					searchPosts.View = saved.View
					if saved.View == "" || time.Since(time.UnixMilli(saved.Timestamp)) > 12*time.Hour {
						searchPosts.View = "trending"
					}
					searchPosts.Date = saved.Date
					if saved.TagLogic == "AND" {
						searchPosts.TagLogic = "AND"
					}
					searchPosts.MyVotesOnly = saved.MyVotes
					searchPosts.MyPostsOnly = saved.MyPosts
					searchPosts.NotMyVotes = saved.NotMyVotes || (c.IsAuthenticated() && !saved.WasAuthenticated)
					if saved.Limit >= 5 && saved.Limit <= 50 {
						limit = saved.Limit
					}
				}
			}
		}

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
				"tags":           tags.Result,
				"countPerStatus": counts.Result,
				"savedFiltersAt": savedFiltersAt,
				"initialFilters": web.Map{
					"query":      searchPosts.Query,
					"view":       searchPosts.View,
					"limit":      limit,
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

		votes := listVotes.Result
		if !entity.Can(c.User(), c.Tenant(), entity.ViewPostVotes) {
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
				"hasReportedPost":   reportedItems.HasReportedPost,
				"dailyLimitReached": reportedItems.CountToday >= c.Tenant().DailyReportLimit(),
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
