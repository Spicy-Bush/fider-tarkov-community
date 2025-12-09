package handlers

import (
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/csv"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// Index is the default home page
func Index() web.HandlerFunc {
	return func(c *web.Context) error {
		c.SetCanonicalURL("")

		getAllTags := &query.GetAllTags{}
		countPerStatus := &query.CountPostPerStatus{}

		if err := bus.Dispatch(c, getAllTags, countPerStatus); err != nil {
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
				"posts":          []interface{}{},
				"tags":           getAllTags.Result,
				"countPerStatus": countPerStatus.Result,
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

		isSubscribed := &query.UserSubscribedTo{PostID: getPost.Result.ID}
		getComments := &query.GetCommentsByPost{Post: getPost.Result}
		getAllTags := &query.GetAllTags{}
		listVotes := &query.ListPostVotes{PostID: getPost.Result.ID, Limit: 24, IncludeEmail: false}
		getAttachments := &query.GetAttachments{Post: getPost.Result}
		getReportReasons := &query.GetReportReasons{}
		if err := bus.Dispatch(c, getAllTags, getComments, listVotes, isSubscribed, getAttachments, getReportReasons); err != nil {
			return c.Failure(err)
		}

		data := web.Map{
			"comments":      getComments.Result,
			"subscribed":    isSubscribed.Result,
			"post":          getPost.Result,
			"tags":          getAllTags.Result,
			"votes":         listVotes.Result,
			"attachments":   getAttachments.Result,
			"reportReasons": getReportReasons.Result,
		}

		if c.User() != nil {
			commentIDs := make([]int, len(getComments.Result))
			for i, comment := range getComments.Result {
				commentIDs[i] = comment.ID
			}

			reportedItems := &query.GetUserReportedItemsOnPost{
				PostID:     getPost.Result.ID,
				CommentIDs: commentIDs,
			}
			countToday := &query.CountUserReportsToday{UserID: c.User().ID}

			if err := bus.Dispatch(c, reportedItems, countToday); err != nil {
				return c.Failure(err)
			}

			dailyLimit := 10
			tenant := c.Tenant()
			if tenant.GeneralSettings != nil && tenant.GeneralSettings.ReportLimitsPerDay > 0 {
				dailyLimit = tenant.GeneralSettings.ReportLimitsPerDay
			}

			data["reportStatus"] = web.Map{
				"hasReportedPost":    reportedItems.HasReportedPost,
				"reportedCommentIds": reportedItems.ReportedCommentIDs,
				"dailyLimitReached":  countToday.Result >= dailyLimit,
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
