package moderation

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

func Run(ctx context.Context) {
	for ctx.Err() == nil {
		worked := false

		if env.IsOpenAIModerationEnabled() {
			var err error
			worked, err = ProcessNext(ctx)

			if err != nil {
				log.Error(ctx, err)
			}
		}

		if !worked {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func ProcessNext(ctx context.Context) (bool, error) {
	claim := &cmd.ClaimModeration{}

	if err := bus.Dispatch(ctx, claim); err != nil {
		return false, err
	}

	if claim.Result == nil {
		return false, nil
	}

	check := claim.Result
	workCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	workCtx = context.WithValue(workCtx, app.TenantCtxKey, &entity.Tenant{ID: check.TenantID})
	images := make([]ImageData, 0, len(check.BlobKeys))
	var failure error

	if delay := time.Until(check.ProviderAvailableAt); delay > 0 {
		failure = &ProviderError{Code: "provider_cooldown", Retryable: true, RetryAfter: delay}
	}

	if failure == nil {
		for _, key := range check.BlobKeys {
			blob := &query.GetBlobByKey{
				Key:                    key,
				AllowUnpublishedAvatar: check.ContentType == "avatar",
			}

			if err := bus.Dispatch(workCtx, blob); err != nil || blob.Result == nil || len(blob.Result.Content) == 0 {
				failure = &ProviderError{Code: "image_unavailable", Retryable: true, Cause: err}
				break
			}

			images = append(images, ImageData{
				Content:     blob.Result.Content,
				ContentType: blob.Result.ContentType,
			})
		}
	}

	var response *ModerationResponse

	if failure == nil {
		response, failure = CallOpenAIModeration(workCtx, check.Text, images)
	}

	finish := &cmd.FinishModeration{Check: *check, Outcome: cmd.ModerationReviewed}

	if failure != nil {
		var providerError *ProviderError
		finish.Outcome = cmd.ModerationRetry
		finish.Error = "moderation_interrupted"
		delay := time.Duration(1<<min(check.Attempts, 10)) * time.Second
		delay += time.Duration(rand.Int63n(int64(delay/2) + 1))

		if stderrors.As(failure, &providerError) {
			finish.Error = providerError.Error()

			if providerError.Cause != nil {
				log.Error(workCtx, providerError.Cause)
			}

			if providerError.Code != "provider_cooldown" {
				finish.CooldownSeconds = int(math.Ceil(providerError.RetryAfter.Seconds()))
			}

			if !providerError.Retryable {
				delay = max(delay, time.Hour)
			}

			delay = max(delay, providerError.RetryAfter)
		}

		finish.RetryAfterSeconds = int(delay.Seconds()) + 1
		log.Error(ctx, fmt.Errorf("moderation %s %d tenant %d attempt %d: %s", check.ContentType, check.ContentID, check.TenantID, check.Attempts, finish.Error))
	} else {
		result, err := json.Marshal(response)

		if err != nil {
			return true, err
		}

		finish.Result = string(result)
		finish.Findings = CheckThresholds(response)
	}

	// Persist the result even if shutdown canceled the provider request.
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()

	if err := bus.Dispatch(finishCtx, finish); err != nil {
		return true, err
	}

	if finish.PublishedName != "" && env.Config.UserList.Enabled {
		update := &cmd.UserListUpdateUser{
			Id:       check.ContentID,
			TenantId: check.TenantID,
			Name:     finish.PublishedName,
		}

		if err := bus.Dispatch(finishCtx, update); err != nil {
			log.Error(finishCtx, err)
		}
	}

	return true, nil
}
