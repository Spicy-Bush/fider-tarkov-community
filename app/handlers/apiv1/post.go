package apiv1

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/metrics"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

// SearchPosts return existing posts based on search criteria
func SearchPosts() web.HandlerFunc {
	return func(c *web.Context) error {
		viewQueryParams := c.QueryParam("view")
		if viewQueryParams == "" {
			viewQueryParams = "all"
		}

		tags := c.QueryParamAsArray("tags")
		var untagged bool
		var filteredTags []string
		for _, t := range tags {
			if t == "untagged" {
				untagged = true
			} else {
				filteredTags = append(filteredTags, t)
			}
		}
		if untagged {
			filteredTags = nil
		}

		clientLimitParam := c.QueryParam("limit")
		clientOffsetParam := c.QueryParam("offset")
		tagLogicParam := c.QueryParam("tagLogic")

		if tagLogicParam != "AND" && tagLogicParam != "OR" {
			tagLogicParam = "OR"
		}

		var clientOffset int
		if clientOffsetParam == "" {
			clientOffset = 0
		} else {
			var err error
			clientOffset, err = strconv.Atoi(clientOffsetParam)
			if err != nil {
				clientOffset = 0
			}
		}

		var effectiveLimit int
		if clientLimitParam == "" || clientLimitParam == "all" {
			if env.Config.Environment == "development" {
				effectiveLimit = 9999
			} else {
				effectiveLimit = 15
			}
		} else {
			clientLimit, err := strconv.Atoi(clientLimitParam)
			if err != nil {
				effectiveLimit = 15
			} else {
				if clientLimit > 15 {
					effectiveLimit = 15
				} else if clientLimit < 5 {
					effectiveLimit = 5
				} else {
					effectiveLimit = clientLimit
				}
			}
		}

		searchQuery := c.QueryParam("query")
		myVotesOnly := false
		if v, err := c.QueryParamAsBool("myvotes"); err == nil {
			myVotesOnly = v
		}
		myPostsOnly := false
		if v, err := c.QueryParamAsBool("myposts"); err == nil {
			myPostsOnly = v
		}
		notMyVotes := false
		if v, err := c.QueryParamAsBool("notmyvotes"); err == nil {
			notMyVotes = v
		}

		statuses := c.QueryParamAsArray("statuses")
		dateFilter := c.QueryParam("date")
		includeCount, _ := c.QueryParamAsBool("includeCount")

		isCacheable := postcache.IsCacheable(
			viewQueryParams,
			myVotesOnly,
			myPostsOnly,
			notMyVotes,
			filteredTags,
			searchQuery,
			untagged,
		) && clientOffset == 0 && len(statuses) == 0 && dateFilter == ""

		tenantID := c.Tenant().ID

		if isCacheable {
			cacheKey := postcache.GetCacheKey(viewQueryParams)
			if cachedIDs, ok := postcache.GetRanking(tenantID, cacheKey); ok && len(cachedIDs) > 0 {
				endIdx := effectiveLimit
				if endIdx > len(cachedIDs) {
					endIdx = len(cachedIDs)
				}
				postIDs := cachedIDs[:endIdx]

				getPostsByIDs := &query.GetPostsByIDs{PostIDs: postIDs}
				if err := bus.Dispatch(c, getPostsByIDs); err != nil {
					return c.Failure(err)
				}

				idToPost := make(map[int]*entity.Post)
				for _, post := range getPostsByIDs.Result {
					idToPost[post.ID] = post
				}

				result := make([]*entity.Post, 0, len(postIDs))
				for _, id := range postIDs {
					if post, ok := idToPost[id]; ok {
						result = append(result, post)
					}
				}

				return c.Ok(result)
			}
		}

		searchPosts := &query.SearchPosts{
			Query:       searchQuery,
			View:        viewQueryParams,
			Limit:       strconv.Itoa(effectiveLimit),
			Offset:      strconv.Itoa(clientOffset),
			Tags:        filteredTags,
			Untagged:    untagged,
			Date:        dateFilter,
			TagLogic:    tagLogicParam,
			MyVotesOnly: myVotesOnly,
			MyPostsOnly: myPostsOnly,
			NotMyVotes:  notMyVotes,
		}

		searchPosts.SetStatusesFromStrings(statuses)

		if err := bus.Dispatch(c, searchPosts); err != nil {
			return c.Failure(err)
		}

		if isCacheable && len(searchPosts.Result) > 0 {
			postIDs := make([]int, len(searchPosts.Result))
			for i, post := range searchPosts.Result {
				postIDs[i] = post.ID
			}
			cacheKey := postcache.GetCacheKey(viewQueryParams)
			postcache.SetRanking(tenantID, cacheKey, postIDs)
		}

		if includeCount && untagged {
			user := c.User()
			if user != nil && (user.IsHelper() || user.IsModerator() || user.IsCollaborator() || user.IsAdministrator()) {
				countQuery := &query.CountUntaggedPosts{Date: dateFilter}
				countQuery.SetStatusesFromStrings(statuses)
				if err := bus.Dispatch(c, countQuery); err == nil {
					c.Response.Header().Set("X-Total-Count", strconv.Itoa(countQuery.Result))
				}
			}
		}

		return c.Ok(searchPosts.Result)
	}
}

// CreatePost creates a new post on current tenant
func CreatePost() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().IsMuted() {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot create new posts.",
			})
		}

		action := new(actions.CreateNewPost)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.UploadImages{Images: action.Attachments, Folder: "attachments"}); err != nil {
				return c.Failure(err)
			}

			newPost := &cmd.AddNewPost{
				Title:       action.Title,
				Description: action.Description,
			}
			if err := bus.Dispatch(c, newPost); err != nil {
				return c.Failure(err)
			}

			setAttachments := &cmd.SetAttachments{Post: newPost.Result, Attachments: action.Attachments}
			addVote := &cmd.AddVote{Post: newPost.Result, User: c.User(), VoteType: enum.VoteTypeUp}
			if err := bus.Dispatch(c, setAttachments, addVote); err != nil {
				return c.Failure(err)
			}

			tagsAssigned := 0
			if env.Config.PostCreationWithTagsEnabled {
				for _, tag := range action.Tags {
					assignTag := &cmd.AssignTag{Tag: tag, Post: newPost.Result}
					if err := bus.Dispatch(c, assignTag); err != nil {
						return c.Failure(err)
					}
					tagsAssigned++
				}
			}

			if tagsAssigned == 0 {
				sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgQueuePostNew, sse.QueueEventPayload{
					PostID: newPost.Result.ID,
				})
			}

			c.Enqueue(tasks.NotifyAboutNewPost(newPost.Result))

			postcache.InvalidateTenantRankings(c.Tenant().ID)
			postcache.InvalidateCountPerStatus(c.Tenant().ID)

			metrics.TotalPosts.Inc()
			return c.Ok(web.Map{
				"id":     newPost.Result.ID,
				"number": newPost.Result.Number,
				"title":  newPost.Result.Title,
				"slug":   newPost.Result.Slug,
			})
		})
	}
}

// GetPost retrieves the existing post by number
func GetPost() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getPost.Result)
	}
}

// UpdatePost updates an existing post of current tenant
func UpdatePost() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UpdatePost)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		getPost := &query.GetPostByNumber{Number: action.Number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		if getPost.Result == nil {
			return c.NotFound()
		}

		if getPost.Result.IsLocked() && !(c.IsAuthenticated() &&
			(c.User().IsCollaborator() || c.User().IsAdministrator())) {
			return c.BadRequest(web.Map{})
		}

		return c.WithTransaction(func() error {
			err := bus.Dispatch(c,
				&cmd.UploadImages{
					Images: action.Attachments,
					Folder: "attachments",
				},
				&cmd.UpdatePost{
					Post:        action.Post,
					Title:       action.Title,
					Description: action.Description,
				},
				&cmd.SetAttachments{
					Post:        action.Post,
					Attachments: action.Attachments,
				},
			)
			if err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

// SetResponse changes current post staff response
func SetResponse() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.SetResponse)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		getPost := &query.GetPostByNumber{Number: action.Number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		prevStatus := getPost.Result.Status

		return c.WithTransaction(func() error {
			var command bus.Msg
			if action.Status == enum.PostDuplicate {
				command = &cmd.MarkPostAsDuplicate{Post: getPost.Result, Original: action.Original, Text: action.Text}
			} else {
				command = &cmd.SetPostResponse{
					Post:   getPost.Result,
					Text:   action.Text,
					Status: action.Status,
				}
			}

			if err := bus.Dispatch(c, command); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutStatusChange(getPost.Result, prevStatus))

			postcache.InvalidateTenantRankings(c.Tenant().ID)
			postcache.InvalidateCountPerStatus(c.Tenant().ID)

			return c.Ok(web.Map{})
		})
	}
}

// DeletePost deletes an existing post of current tenant
func DeletePost() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.DeletePost)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			err := bus.Dispatch(c, &cmd.SetPostResponse{
				Post:   action.Post,
				Text:   action.Text,
				Status: enum.PostDeleted,
			})
			if err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.TriggerDeleteWebhook(action.Post))

			postcache.InvalidateTenantRankings(c.Tenant().ID)
			postcache.InvalidateCountPerStatus(c.Tenant().ID)

			return c.Ok(web.Map{})
		})
	}
}

// ListComments returns a list of all comments of a post
func ListComments() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		getComments := &query.GetCommentsByPost{Post: getPost.Result}
		if err := bus.Dispatch(c, getComments); err != nil {
			return c.Failure(err)
		}

		// the content of the comment needs to be sanitized before it is returned
		for _, comment := range getComments.Result {
			comment.Content = markdown.StripMentionMetaData(comment.Content)
		}

		return c.Ok(getComments.Result)
	}
}

// GetPostAttachments returns a list of attachments for a post
func GetPostAttachments() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		getAttachments := &query.GetAttachments{Post: getPost.Result}
		if err := bus.Dispatch(c, getAttachments); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getAttachments.Result)
	}
}

// GetComment returns a single comment by its ID
func GetComment() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		commentByID := &query.GetCommentByID{CommentID: id}
		if err := bus.Dispatch(c, commentByID); err != nil {
			return c.Failure(err)
		}

		commentByID.Result.Content = markdown.StripMentionMetaData(commentByID.Result.Content)

		return c.Ok(commentByID.Result)
	}
}

// ToggleReaction adds or removes a reaction on a comment
func ToggleReaction() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().IsMuted() {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot add reactions.",
			})
		}

		action := new(actions.ToggleCommentReaction)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			toggleReaction := &cmd.ToggleCommentReaction{
				Comment: action.Comment,
				Emoji:   action.Reaction,
				User:    c.User(),
			}
			if err := bus.Dispatch(c, toggleReaction); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{
				"added": toggleReaction.Result,
			})
		})
	}
}

// PostComment creates a new comment on given post
func PostComment() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().IsMuted() {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot post comments.",
			})
		}

		action := new(actions.AddNewComment)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		getPost := &query.GetPostByNumber{Number: action.Number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		if getPost.Result == nil {
			return c.NotFound()
		}

		if getPost.Result.IsLocked() && !(c.IsAuthenticated() &&
			(c.User().IsCollaborator() || c.User().IsAdministrator())) {
			return c.BadRequest(web.Map{})
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.UploadImages{Images: action.Attachments, Folder: "attachments"}); err != nil {
				return c.Failure(err)
			}

			contentToSave := entity.CommentString(action.Content).FormatMentionJson(func(mention entity.Mention) string {
				nameJSON, _ := json.Marshal(mention.Name)
				return fmt.Sprintf(`{"id":%d,"name":%s}`, mention.ID, string(nameJSON))
			})

			addNewComment := &cmd.AddNewComment{
				Post:    getPost.Result,
				Content: contentToSave,
			}
			if err := bus.Dispatch(c, addNewComment); err != nil {
				return c.Failure(err)
			}

			if err := bus.Dispatch(c, &cmd.SetAttachments{
				Post:        getPost.Result,
				Comment:     addNewComment.Result,
				Attachments: action.Attachments,
			}); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutNewComment(addNewComment.Result, getPost.Result))

			if getPost.Result.Status == enum.PostArchived {
				unarchiveCmd := &cmd.UnarchivePost{Post: getPost.Result, Reason: "New comment"}
				if err := bus.Dispatch(c, unarchiveCmd); err != nil {
					return c.Failure(err)
				}
				postcache.InvalidateCountPerStatus(c.Tenant().ID)
			}

			postcache.InvalidateTenantRankings(c.Tenant().ID)

			metrics.TotalComments.Inc()
			
			attachmentBKeys := make([]string, 0)
			for _, att := range action.Attachments {
				if att.BlobKey != "" && !att.Remove {
					attachmentBKeys = append(attachmentBKeys, att.BlobKey)
				}
			}
			
			return c.Ok(web.Map{
				"id":          addNewComment.Result.ID,
				"attachments": attachmentBKeys,
			})
		})
	}
}

// UpdateComment changes an existing comment with new content
func UpdateComment() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().IsMuted() {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot edit comments.",
			})
		}

		action := new(actions.EditComment)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		getPost := &query.GetPostByID{PostID: action.Post.ID}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		if getPost.Result == nil {
			return c.NotFound()
		}

		if getPost.Result.IsLocked() && !(c.IsAuthenticated() &&
			(c.User().IsCollaborator() || c.User().IsAdministrator())) {
			return c.BadRequest(web.Map{})
		}

		return c.WithTransaction(func() error {
			contentToSave := entity.CommentString(action.Content).FormatMentionJson(func(mention entity.Mention) string {
				nameJSON, _ := json.Marshal(mention.Name)
				return fmt.Sprintf(`{"id":%d,"name":%s}`, mention.ID, string(nameJSON))
			})

			err := bus.Dispatch(c,
				&cmd.UploadImages{
					Images: action.Attachments,
					Folder: "attachments",
				},
				&cmd.UpdateComment{
					CommentID: action.ID,
					Content:   contentToSave,
				},
				&cmd.SetAttachments{
					Post:        action.Post,
					Comment:     action.Comment,
					Attachments: action.Attachments,
				},
			)
			if err != nil {
				return c.Failure(err)
			}

			// Update the content
			c.Enqueue(tasks.NotifyAboutUpdatedComment(contentToSave, getPost.Result, action.ID))

			return c.Ok(web.Map{})
		})
	}
}

// DeleteComment deletes an existing comment by its ID
func DeleteComment() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.DeleteComment)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		getPost := &query.GetPostByNumber{Number: action.PostNumber}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		return c.WithTransaction(func() error {
			err := bus.Dispatch(c, &cmd.DeleteComment{
				CommentID: action.CommentID,
			})
			if err != nil {
				return c.Failure(err)
			}

			if getPost.Result != nil {
				postcache.InvalidateTenantRankings(c.Tenant().ID)
			}

			return c.Ok(web.Map{})
		})
	}
}

// AddDownVote adds current user to given post list with -1 for votetype
func AddDownVote() web.HandlerFunc {
	return func(c *web.Context) error {
		err := addOrRemove(c, func(post *entity.Post, user *entity.User) bus.Msg {
			return &cmd.AddVote{Post: post, User: user, VoteType: enum.VoteTypeDown}
		})

		if err == nil {
			// TODO: figure out prometheus metrics for downvotes later
			metrics.TotalVotes.Inc()
		}

		return err
	}
}

// AddVote adds current user to given post list of votes
func AddVote() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		if getPost.Result.IsLocked() && !(c.IsAuthenticated() &&
			(c.User().IsCollaborator() || c.User().IsAdministrator())) {
			return c.BadRequest(web.Map{})
		}

		return c.WithTransaction(func() error {
			voteCmd := &cmd.AddVote{Post: getPost.Result, User: c.User(), VoteType: enum.VoteTypeUp}
			if err := bus.Dispatch(c, voteCmd); err != nil {
				return c.Failure(err)
			}

			if getPost.Result.Status == enum.PostArchived && getPost.Result.ArchivedSettings != nil {
				countVotes := &query.CountVotesSinceArchive{
					PostID:     getPost.Result.ID,
					ArchivedAt: getPost.Result.ArchivedSettings.ArchivedAt,
				}
				if err := bus.Dispatch(c, countVotes); err != nil {
					return c.Failure(err)
				}

				if countVotes.Result > 10 {
					unarchiveCmd := &cmd.UnarchivePost{Post: getPost.Result, Reason: "Vote threshold exceeded"}
					if err := bus.Dispatch(c, unarchiveCmd); err != nil {
						return c.Failure(err)
					}
					postcache.InvalidateCountPerStatus(c.Tenant().ID)
				}
			}

			postcache.InvalidateTenantRankings(c.Tenant().ID)
			metrics.TotalVotes.Inc()

			return c.Ok(web.Map{})
		})
	}
}

// RemoveVote removes current user from given post list of votes
func RemoveVote() web.HandlerFunc {
	return func(c *web.Context) error {
		return addOrRemove(c, func(post *entity.Post, user *entity.User) bus.Msg {
			return &cmd.RemoveVote{Post: post, User: user}
		})
	}
}

func ToggleVote() web.HandlerFunc {
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

		if getPost.Result.IsLocked() && !(c.IsAuthenticated() &&
			(c.User().IsCollaborator() || c.User().IsAdministrator())) {
			return c.BadRequest(web.Map{})
		}

		listVotes := &query.ListPostVotes{PostID: getPost.Result.ID}
		if err := bus.Dispatch(c, listVotes); err != nil {
			return c.Failure(err)
		}

		var existingVote enum.VoteType = 0
		for _, vote := range listVotes.Result {
			if vote.User.ID == c.User().ID {
				existingVote = vote.VoteType
				break
			}
		}

		// If they had UP, switch to DOWN
		// If they had DOWN, switch to UP
		// If none, set it to UP by default
		var newVote enum.VoteType
		switch existingVote {
		case enum.VoteTypeUp:
			newVote = enum.VoteTypeDown
		case enum.VoteTypeDown:
			newVote = enum.VoteTypeUp
		default:
			newVote = enum.VoteTypeUp
		}

		return c.WithTransaction(func() error {
			err = bus.Dispatch(c, &cmd.AddVote{Post: getPost.Result, User: c.User(), VoteType: newVote})
			if err != nil {
				return c.Failure(err)
			}

			postcache.InvalidateTenantRankings(c.Tenant().ID)

			metrics.TotalVotes.Inc()
			return c.Ok(web.Map{"voted": (newVote == enum.VoteTypeUp)})
		})
	}
}

// Subscribe adds current user to list of subscribers of given post
func Subscribe() web.HandlerFunc {
	return func(c *web.Context) error {
		return addOrRemove(c, func(post *entity.Post, user *entity.User) bus.Msg {
			return &cmd.AddSubscriber{Post: post, User: user}
		})
	}
}

// Unsubscribe removes current user from list of subscribers of given post
func Unsubscribe() web.HandlerFunc {
	return func(c *web.Context) error {
		return addOrRemove(c, func(post *entity.Post, user *entity.User) bus.Msg {
			return &cmd.RemoveSubscriber{Post: post, User: user}
		})
	}
}

// ListVotes returns a list of all votes on given post
func ListVotes() web.HandlerFunc {
	return func(c *web.Context) error {
		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		listVotes := &query.ListPostVotes{PostID: getPost.Result.ID, IncludeEmail: false}
		err = bus.Dispatch(c, listVotes)
		if err != nil {
			return c.Failure(err)
		}

		return c.Ok(listVotes.Result)
	}
}

func addOrRemove(c *web.Context, getCommand func(post *entity.Post, user *entity.User) bus.Msg) error {
	number, err := c.ParamAsInt("number")
	if err != nil {
		return c.NotFound()
	}

	getPost := &query.GetPostByNumber{Number: number}
	if err := bus.Dispatch(c, getPost); err != nil {
		return c.Failure(err)
	}

	if getPost.Result.IsLocked() && !(c.IsAuthenticated() &&
		(c.User().IsCollaborator() || c.User().IsAdministrator())) {
		return c.BadRequest(web.Map{})
	}

	return c.WithTransaction(func() error {
		cmd := getCommand(getPost.Result, c.User())
		err = bus.Dispatch(c, cmd)
		if err != nil {
			return c.Failure(err)
		}

		postcache.InvalidateTenantRankings(c.Tenant().ID)

		return c.Ok(web.Map{})
	})
}

func LockOrUnlockPost() web.HandlerFunc {
	return func(c *web.Context) error {
		isLocking := c.Request.Method == "PUT"

		if isLocking {
			action := new(actions.LockPost)
			if result := c.BindTo(action); !result.Ok {
				return c.HandleValidation(result)
			}

			return c.WithTransaction(func() error {
				lockPost := &cmd.LockPost{
					Post:        action.Post,
					LockMessage: action.LockMessage,
				}
				if err := bus.Dispatch(c, lockPost); err != nil {
					return c.Failure(err)
				}
				return c.Ok(web.Map{})
			})
		} else if c.Request.Method == "DELETE" {
			action := new(actions.UnlockPost)
			if result := c.BindTo(action); !result.Ok {
				return c.HandleValidation(result)
			}

			return c.WithTransaction(func() error {
				unlockPost := &cmd.UnlockPost{
					Post: action.Post,
				}
				if err := bus.Dispatch(c, unlockPost); err != nil {
					return c.Failure(err)
				}
				return c.Ok(web.Map{})
			})
		}

		return c.BadRequest(web.Map{})
	}
}
