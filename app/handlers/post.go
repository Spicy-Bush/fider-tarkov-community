package handlers

import (
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

		var tags []*entity.Tag
		var countPerStatus map[enum.PostStatus]int

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

		var pfilterValue string
		if cookie, err := c.Request.Cookie("pfilter"); err == nil {
			pfilterValue = cookie.Value
		}
		pf := web.DecodePFilter(pfilterValue, c.User() != nil)
		tagSlugs := tagIDsToSlugs(pf.Tags, tags)
		statuses := statusIDsToStatuses(pf.Statuses)

		searchPosts := &query.SearchPosts{
			View:        pf.View,
			Limit:       "20",
			Tags:        tagSlugs,
			Statuses:    statuses,
			MyVotesOnly: pf.MyVotes,
			MyPostsOnly: pf.MyPosts,
			NotMyVotes:  pf.NotMyVotes,
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
				"pfilter":        pf,
			},
		})
	}
}

func tagIDsToSlugs(ids []int, tags []*entity.Tag) []string {
	if len(ids) == 0 {
		return nil
	}
	idSet := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	slugs := make([]string, 0, len(ids))
	for _, tag := range tags {
		if _, ok := idSet[tag.ID]; ok {
			slugs = append(slugs, tag.Slug)
		}
	}
	return slugs
}

func statusIDsToStatuses(ids []int) []enum.PostStatus {
	if len(ids) == 0 {
		return nil
	}
	out := make([]enum.PostStatus, 0, len(ids))
	for _, id := range ids {
		switch enum.PostStatus(id) {
		case enum.PostOpen, enum.PostStarted, enum.PostCompleted, enum.PostDeclined, enum.PostPlanned, enum.PostDuplicate, enum.PostDeleted, enum.PostArchived:
			out = append(out, enum.PostStatus(id))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
		getAttachments := &query.GetAttachments{Post: getPost.Result}
		getReportReasons := &query.GetReportReasons{}
		if err := bus.Dispatch(c, getAllTags, getComments, isSubscribed, getAttachments, getReportReasons); err != nil {
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
			"comments":      getComments.Result,
			"subscribed":    isSubscribed.Result,
			"post":          getPost.Result,
			"tags":          getAllTags.Result,
			"votes":         votes,
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
