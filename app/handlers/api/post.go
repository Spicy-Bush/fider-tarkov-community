package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/metrics"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

// SearchPosts return existing posts based on search criteria
func SearchPosts() web.HandlerFunc {
	return func(c *web.Context) error {
		var ids []int
		if selection, present := c.Request.URL.Query()["ids"]; present {
			var err error
			ids, err = parseRecordIDs(selection)
			if err != nil {
				return c.BadRequest(web.Map{"message": "Invalid post selection."})
			}

			c.Response.Header().Set("Cache-Control", "private, no-store")
		}

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
		isPrivileged := entity.Can(c.User(), c.Tenant(), entity.ManageQueue)
		maxLimit := 15
		if isPrivileged {
			maxLimit = 50
		}

		if clientLimitParam == "" || clientLimitParam == "all" {
			effectiveLimit = maxLimit
		} else {
			clientLimit, err := strconv.Atoi(clientLimitParam)
			if err != nil {
				effectiveLimit = maxLimit
			} else {
				if clientLimit > maxLimit {
					effectiveLimit = maxLimit
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

		if len(ids) > 0 {
			effectiveLimit = len(ids)
			clientOffset = 0
		}

		searchPosts := &query.SearchPosts{
			IDs:         ids,
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

		if includeCount && untagged {
			if isPrivileged {
				countQuery := &query.CountUntaggedPosts{Date: dateFilter}
				countQuery.SetStatusesFromStrings(statuses)
				if err := bus.Dispatch(c, countQuery); err == nil {
					c.Response.Header().Set("X-Total-Count", strconv.Itoa(countQuery.Result))
				}
			}
		}

		if token := c.QueryParam("sponsorPage"); token != "" {
			page, err := adsselect.ReadPage(token, env.Config.JWTSecret, c.Tenant().ID, c.SessionID(), time.Now())
			if err == nil && page.Kind == "home" && len(searchPosts.Result) > 0 {
				page.PostIDs = make([]int, len(searchPosts.Result))
				for index, post := range searchPosts.Result {
					page.PostIDs[index] = post.ID
				}

				postToken, err := page.Token(env.Config.JWTSecret)
				if err != nil {
					return err
				}
				posts := make([]*entity.Post, len(searchPosts.Result))
				for index, post := range searchPosts.Result {
					copy := *post
					copy.SponsorPage = postToken
					posts[index] = &copy
				}
				searchPosts.Result = posts
			}
		}

		return c.Ok(searchPosts.Result)
	}
}

// CreatePost creates a new post on current tenant
func CreatePost() web.HandlerFunc {
	return func(c *web.Context) error {
		if !c.IsAuthenticated() {
			return c.Unauthorized()
		}

		action := new(actions.CreateNewPost)
		if err := c.Bind(action); err != nil {
			return c.BadRequest(web.Map{
				"message": "Invalid post submission.",
			})
		}

		original, err := json.Marshal(action)
		if err != nil {
			return err
		}

		fingerprint := fmt.Sprintf("%x", sha256.Sum256(original))
		var created *entity.Post
		var tagsAssigned int

		submission := &cmd.SubmitPost{
			SubmissionID: action.SubmissionID,
			Fingerprint:  fingerprint,
			BaseURL:      web.BaseURL(c),
			Attachments:  action.Attachments,
		}

		submission.Validate = func(ctx context.Context) error {
			user := ctx.Value(app.UserCtxKey).(*entity.User)
			if user.IsMuted() {
				return validate.Failed("You are currently muted and cannot create new posts.")
			}

			if err := action.OnPreExecute(ctx); err != nil {
				return err
			}

			if !action.IsAuthorized(ctx, user) {
				return validate.Unauthorized()
			}

			validation := action.Validate(ctx, user)
			if !validation.Ok {
				return validation
			}
			return nil
		}

		submission.Create = func(ctx context.Context) (*entity.Post, error) {
			user := ctx.Value(app.UserCtxKey).(*entity.User)
			newPost := &cmd.AddNewPost{
				Title:       action.Title,
				Description: action.Description,
				Attachments: action.Attachments,
			}

			if err := bus.Dispatch(ctx, newPost); err != nil {
				return nil, err
			}

			created = newPost.Result
			addVote := &cmd.AddVote{
				Post:     created,
				User:     user,
				VoteType: enum.VoteTypeUp,
			}

			if err := bus.Dispatch(ctx, addVote); err != nil {
				return nil, err
			}

			for _, tag := range action.Tags {
				assignTag := &cmd.AssignTag{
					Tag:  tag,
					Post: created,
				}

				if err := bus.Dispatch(ctx, assignTag); err != nil {
					return nil, err
				}

				tagsAssigned++
			}

			moderation := &cmd.ScheduleModeration{
				ContentType: "post",
				ContentID:   created.ID,
			}

			if err := bus.Dispatch(ctx, moderation); err != nil {
				return nil, err
			}

			return created, nil
		}

		err = bus.Dispatch(c, submission)
		if err != nil {
			return c.Failure(err)
		}

		if created != nil {
			if tagsAssigned == 0 {
				sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgQueuePostNew, sse.QueueEventPayload{
					PostID: created.ID,
				})
			}

			metrics.TotalPosts.Inc()
		}

		return c.Ok(submission.Result)
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

		err := bus.Dispatch(c, &cmd.UpdatePost{
			Post:        action.Post,
			Title:       action.Title,
			Description: action.Description,
			Attachments: action.Attachments,
		})
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

		return c.WithTransaction(func() error {
			var command bus.Msg
			if *action.Status == enum.PostDuplicate {
				command = &cmd.MarkPostAsDuplicate{Post: getPost.Result, Original: action.Original, Text: action.Text}
			} else {
				command = &cmd.SetPostResponse{
					Post:   getPost.Result,
					Text:   action.Text,
					Status: *action.Status,
				}
			}

			if err := bus.Dispatch(c, command); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutStatusChange(getPost.Result, prevStatus))

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

			return c.Ok(web.Map{})
		})
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

		getAttachments := &query.GetPostAttachments{PostID: getPost.Result.ID}
		if err := bus.Dispatch(c, getAttachments); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getAttachments.Result)
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
		preview := c.QueryParam("preview") == "true"
		if !preview && !entity.Can(c.User(), c.Tenant(), entity.ViewPostVotes) {
			return c.Forbidden()
		}

		number, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: number}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.Failure(err)
		}

		listVotes := &query.ListPostVotes{PostID: getPost.Result.ID, Preview: preview}
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

	if !getPost.Result.AllowedActions(c.User(), c.Tenant(), time.Now()).Follow {
		return c.Forbidden()
	}

	return c.WithTransaction(func() error {
		cmd := getCommand(getPost.Result, c.User())
		err = bus.Dispatch(c, cmd)
		if err != nil {
			return c.Failure(err)
		}

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
