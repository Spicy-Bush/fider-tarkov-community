package apiv1

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/metrics"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func isUserMuted(ctx *web.Context) (bool, error) {
	standing := &query.GetUserProfileStanding{
		UserID: ctx.User().ID,
		Result: struct {
			Warnings []struct {
				ID        int        `json:"id"`
				Reason    string     `json:"reason"`
				CreatedAt time.Time  `json:"createdAt"`
				ExpiresAt *time.Time `json:"expiresAt,omitempty"`
			} `json:"warnings"`
			Mutes []struct {
				ID        int        `json:"id"`
				Reason    string     `json:"reason"`
				CreatedAt time.Time  `json:"createdAt"`
				ExpiresAt *time.Time `json:"expiresAt,omitempty"`
			} `json:"mutes"`
		}{},
	}
	err := bus.Dispatch(ctx, standing)
	if err != nil {
		return false, err
	}

	now := time.Now()
	for _, mute := range standing.Result.Mutes {
		if mute.ExpiresAt == nil || mute.ExpiresAt.After(now) {
			return true, nil
		}
	}
	return false, nil
}

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

		searchPosts := &query.SearchPosts{
			Query:    c.QueryParam("query"),
			View:     viewQueryParams,
			Limit:    strconv.Itoa(effectiveLimit),
			Offset:   strconv.Itoa(clientOffset),
			Tags:     filteredTags,
			Untagged: untagged,
			Date:     c.QueryParam("date"),
			TagLogic: tagLogicParam,
		}

		if myVotesOnly, err := c.QueryParamAsBool("myvotes"); err == nil {
			searchPosts.MyVotesOnly = myVotesOnly
		}

		if myPostsOnly, err := c.QueryParamAsBool("myposts"); err == nil {
			searchPosts.MyPostsOnly = myPostsOnly
		}

		if notMyVotes, err := c.QueryParamAsBool("notmyvotes"); err == nil {
			searchPosts.NotMyVotes = notMyVotes
		}

		searchPosts.SetStatusesFromStrings(c.QueryParamAsArray("statuses"))

		if err := bus.Dispatch(c, searchPosts); err != nil {
			return c.Failure(err)
		}
		return c.Ok(searchPosts.Result)
	}
}

// CreatePost creates a new post on current tenant
func CreatePost() web.HandlerFunc {
	return func(c *web.Context) error {
		isMuted, err := isUserMuted(c)
		if err != nil {
			return c.Failure(err)
		}
		if isMuted {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot create new posts.",
			})
		}

		action := new(actions.CreateNewPost)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		if err := bus.Dispatch(c, &cmd.UploadImages{Images: action.Attachments, Folder: "attachments"}); err != nil {
			return c.Failure(err)
		}

		newPost := &cmd.AddNewPost{
			Title:       action.Title,
			Description: action.Description,
		}
		err = bus.Dispatch(c, newPost)
		if err != nil {
			return c.Failure(err)
		}

		setAttachments := &cmd.SetAttachments{Post: newPost.Result, Attachments: action.Attachments}
		addVote := &cmd.AddVote{Post: newPost.Result, User: c.User(), VoteType: enum.VoteTypeUp}
		if err = bus.Dispatch(c, setAttachments, addVote); err != nil {
			return c.Failure(err)
		}

		if env.Config.PostCreationWithTagsEnabled {
			for _, tagSlug := range action.TagSlugs {
				getTag := &query.GetTagBySlug{Slug: tagSlug}
				if err := bus.Dispatch(c, getTag); err != nil {
					return c.Failure(err)
				}
				assignTag := &cmd.AssignTag{Tag: getTag.Result, Post: newPost.Result}
				if err := bus.Dispatch(c, assignTag); err != nil {
					return c.Failure(err)
				}
			}
		}

		c.Enqueue(tasks.NotifyAboutNewPost(newPost.Result))

		metrics.TotalPosts.Inc()
		return c.Ok(web.Map{
			"id":     newPost.Result.ID,
			"number": newPost.Result.Number,
			"title":  newPost.Result.Title,
			"slug":   newPost.Result.Slug,
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

		var command bus.Msg
		if action.Status == enum.PostDuplicate {
			command = &cmd.MarkPostAsDuplicate{Post: getPost.Result, Original: action.Original}
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

		return c.Ok(web.Map{})
	}
}

// DeletePost deletes an existing post of current tenant
func DeletePost() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.DeletePost)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		err := bus.Dispatch(c, &cmd.SetPostResponse{
			Post:   action.Post,
			Text:   action.Text,
			Status: enum.PostDeleted,
		})
		if err != nil {
			return c.Failure(err)
		}

		c.Enqueue(tasks.TriggerDeleteWebhook(action.Post))

		return c.Ok(web.Map{})
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
		isMuted, err := isUserMuted(c)
		if err != nil {
			return c.Failure(err)
		}
		if isMuted {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot add reactions.",
			})
		}

		action := new(actions.ToggleCommentReaction)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

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
	}
}

// PostComment creates a new comment on given post
func PostComment() web.HandlerFunc {
	return func(c *web.Context) error {
		isMuted, err := isUserMuted(c)
		if err != nil {
			return c.Failure(err)
		}
		if isMuted {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot post comments.",
			})
		}

		action := new(actions.AddNewComment)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		if err := bus.Dispatch(c, &cmd.UploadImages{Images: action.Attachments, Folder: "attachments"}); err != nil {
			return c.Failure(err)
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

		addNewComment := &cmd.AddNewComment{
			Post:    getPost.Result,
			Content: action.Content,
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

		metrics.TotalComments.Inc()
		return c.Ok(web.Map{
			"id": addNewComment.Result.ID,
		})
	}
}

// UpdateComment changes an existing comment with new content
func UpdateComment() web.HandlerFunc {
	return func(c *web.Context) error {
		isMuted, err := isUserMuted(c)
		if err != nil {
			return c.Failure(err)
		}
		if isMuted {
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

		contentToSave := entity.CommentString(action.Content).FormatMentionJson(func(mention entity.Mention) string {
			return fmt.Sprintf(`{"id":%d,"name":"%s"}`, mention.ID, mention.Name)
		})

		err = bus.Dispatch(c,
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
	}
}

// DeleteComment deletes an existing comment by its ID
func DeleteComment() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.DeleteComment)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		err := bus.Dispatch(c, &cmd.DeleteComment{
			CommentID: action.CommentID,
		})
		if err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{})
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
		err := addOrRemove(c, func(post *entity.Post, user *entity.User) bus.Msg {
			return &cmd.AddVote{Post: post, User: user, VoteType: enum.VoteTypeUp}
		})

		if err == nil {
			metrics.TotalVotes.Inc()
		}

		return err
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

		err = bus.Dispatch(c, &cmd.AddVote{Post: getPost.Result, User: c.User(), VoteType: newVote})
		if err != nil {
			return c.Failure(err)
		}
		metrics.TotalVotes.Inc()
		return c.Ok(web.Map{"voted": (newVote == enum.VoteTypeUp)})
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

	cmd := getCommand(getPost.Result, c.User())
	err = bus.Dispatch(c, cmd)
	if err != nil {
		return c.Failure(err)
	}

	return c.Ok(web.Map{})
}

func LockOrUnlockPost() web.HandlerFunc {
	return func(c *web.Context) error {
		isLocking := c.Request.Method == "PUT"

		if isLocking {
			action := new(actions.LockPost)
			if result := c.BindTo(action); !result.Ok {
				return c.HandleValidation(result)
			}

			lockPost := &cmd.LockPost{
				Post:        action.Post,
				LockMessage: action.LockMessage,
			}
			if err := bus.Dispatch(c, lockPost); err != nil {
				return c.Failure(err)
			}
		} else if c.Request.Method == "DELETE" {
			action := new(actions.UnlockPost)
			if result := c.BindTo(action); !result.Ok {
				return c.HandleValidation(result)
			}

			unlockPost := &cmd.UnlockPost{
				Post: action.Post,
			}
			if err := bus.Dispatch(c, unlockPost); err != nil {
				return c.Failure(err)
			}
		} else {
			return c.BadRequest(web.Map{})
		}

		return c.Ok(web.Map{})
	}
}
