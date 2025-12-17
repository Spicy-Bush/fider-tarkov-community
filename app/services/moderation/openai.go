package moderation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

const OpenAIModerationURL = "https://api.openai.com/v1/moderations"

type ImageURLInput struct {
	URL string `json:"url"`
}

type ModerationInput struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *ImageURLInput `json:"image_url,omitempty"`
}

type ImageData struct {
	Content     []byte
	ContentType string
}

type ModerationRequest struct {
	Model string            `json:"model"`
	Input []ModerationInput `json:"input"`
}

type ModerationResult struct {
	Flagged    bool                `json:"flagged"`
	Categories map[string]bool     `json:"categories"`
	Scores     map[string]float64  `json:"category_scores"`
	InputTypes map[string][]string `json:"category_applied_input_types,omitempty"`
}

type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

type FlaggedCategory struct {
	Category string
	Score    float64
}

func CallOpenAIModeration(ctx context.Context, text string, images []ImageData) (*ModerationResponse, error) {
	if env.Config.OpenAI.APIKey == "" {
		return nil, errors.New("OpenAI API key not configured")
	}

	if text == "" && len(images) == 0 {
		return nil, errors.New("no content to moderate")
	}

	combinedResponse := &ModerationResponse{
		Results: []ModerationResult{},
	}

	if text != "" {
		textResponse, err := callSingleModeration(ctx, []ModerationInput{{Type: "text", Text: text}})
		if err != nil {
			return nil, err
		}
		combinedResponse.ID = textResponse.ID
		combinedResponse.Model = textResponse.Model
		combinedResponse.Results = append(combinedResponse.Results, textResponse.Results...)
	}

	for _, img := range images {
		base64Data := base64.StdEncoding.EncodeToString(img.Content)
		dataURL := fmt.Sprintf("data:%s;base64,%s", img.ContentType, base64Data)
		imgInput := ModerationInput{
			Type:     "image_url",
			ImageURL: &ImageURLInput{URL: dataURL},
		}

		imgResponse, err := callSingleModeration(ctx, []ModerationInput{imgInput})
		if err != nil {
			return nil, err
		}
		if combinedResponse.ID == "" {
			combinedResponse.ID = imgResponse.ID
			combinedResponse.Model = imgResponse.Model
		}
		combinedResponse.Results = append(combinedResponse.Results, imgResponse.Results...)
	}

	return combinedResponse, nil
}

func callSingleModeration(ctx context.Context, inputs []ModerationInput) (*ModerationResponse, error) {
	request := ModerationRequest{
		Model: "omni-moderation-latest",
		Input: inputs,
	}

	jsonContent, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal moderation request")
	}

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		httpReq := &cmd.HTTPRequest{
			URL:    OpenAIModerationURL,
			Body:   bytes.NewBuffer(jsonContent),
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer " + env.Config.OpenAI.APIKey,
			},
		}

		if err := bus.Dispatch(ctx, httpReq); err != nil {
			lastErr = errors.Wrap(err, "failed to call OpenAI moderation API")
			continue
		}

		if httpReq.ResponseStatusCode == 429 {
			if isOutOfTokens(httpReq) {
				resetTokens := httpReq.ResponseHeader.Get("x-ratelimit-reset-tokens")
				return nil, errors.New("OpenAI token quota exhausted, resets in %s", resetTokens)
			}
			waitTime, resetHeader := getRateLimitWaitTime(httpReq)
			log.Error(ctx, errors.New("OpenAI rate limit hit, waiting %s before retry (attempt %d/%d)", resetHeader, attempt+1, maxRetries))
			time.Sleep(waitTime)
			lastErr = errors.New("rate limited")
			continue
		}

		if httpReq.ResponseStatusCode != 200 {
			return nil, errors.New("OpenAI moderation API returned status %d: %s", httpReq.ResponseStatusCode, string(httpReq.ResponseBody))
		}

		checkRateLimitHeaders(ctx, httpReq)

		var response ModerationResponse
		if err := json.Unmarshal(httpReq.ResponseBody, &response); err != nil {
			return nil, errors.Wrap(err, "failed to parse moderation response")
		}

		return &response, nil
	}

	return nil, errors.Wrap(lastErr, "failed after %d retries", maxRetries)
}

func getRateLimitWaitTime(httpReq *cmd.HTTPRequest) (time.Duration, string) {
	if httpReq.ResponseHeader == nil {
		return 5 * time.Second, "5s (default)"
	}

	resetRequests := httpReq.ResponseHeader.Get("x-ratelimit-reset-requests")
	if resetRequests != "" {
		if d := parseDuration(resetRequests); d > 0 {
			return d + 100*time.Millisecond, resetRequests
		}
	}

	resetTokens := httpReq.ResponseHeader.Get("x-ratelimit-reset-tokens")
	if resetTokens != "" {
		if d := parseDuration(resetTokens); d > 0 {
			return d + 100*time.Millisecond, resetTokens
		}
	}

	return 5 * time.Second, "5s (default)"
}

func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	d, err := time.ParseDuration(s)
	if err == nil {
		return d
	}

	total := time.Duration(0)
	current := ""

	for _, c := range s {
		if c >= '0' && c <= '9' || c == '.' {
			current += string(c)
		} else {
			if current != "" {
				val, _ := strconv.ParseFloat(current, 64)
				switch c {
				case 'h':
					total += time.Duration(val * float64(time.Hour))
				case 'm':
					total += time.Duration(val * float64(time.Minute))
				case 's':
					total += time.Duration(val * float64(time.Second))
				}
				current = ""
			}
		}
	}

	return total
}

func checkRateLimitHeaders(ctx context.Context, httpReq *cmd.HTTPRequest) {
	if httpReq.ResponseHeader == nil {
		return
	}

	remainingRequests := httpReq.ResponseHeader.Get("x-ratelimit-remaining-requests")
	if remainingRequests != "" {
		remaining, _ := strconv.Atoi(remainingRequests)
		if remaining <= 5 {
			log.Error(ctx, errors.New("OpenAI rate limit warning: only %d requests remaining", remaining))
		}
	}

	remainingTokens := httpReq.ResponseHeader.Get("x-ratelimit-remaining-tokens")
	if remainingTokens != "" {
		remaining, _ := strconv.Atoi(remainingTokens)
		if remaining <= 1000 {
			log.Error(ctx, errors.New("OpenAI rate limit warning: only %d tokens remaining", remaining))
		}
	}
}

func isOutOfTokens(httpReq *cmd.HTTPRequest) bool {
	if httpReq.ResponseHeader == nil {
		return false
	}

	remainingTokens := httpReq.ResponseHeader.Get("x-ratelimit-remaining-tokens")
	if remainingTokens != "" {
		remaining, _ := strconv.Atoi(remainingTokens)
		return remaining == 0
	}
	return false
}

func CheckThresholds(response *ModerationResponse) []FlaggedCategory {
	flagged := []FlaggedCategory{}

	if len(response.Results) == 0 {
		return flagged
	}

	for _, result := range response.Results {
		if score, ok := result.Scores["sexual"]; ok && score >= env.Config.OpenAI.SexualThreshold {
			flagged = append(flagged, FlaggedCategory{
				Category: "sexual",
				Score:    score,
			})
		}

		if score, ok := result.Scores["sexual/minors"]; ok && score >= env.Config.OpenAI.SexualThreshold {
			flagged = append(flagged, FlaggedCategory{
				Category: "sexual/minors",
				Score:    score,
			})
		}

		if score, ok := result.Scores["self-harm"]; ok && score >= env.Config.OpenAI.SelfHarmThreshold {
			flagged = append(flagged, FlaggedCategory{
				Category: "self-harm",
				Score:    score,
			})
		}

		if score, ok := result.Scores["self-harm/intent"]; ok && score >= env.Config.OpenAI.SelfHarmThreshold {
			flagged = append(flagged, FlaggedCategory{
				Category: "self-harm/intent",
				Score:    score,
			})
		}

		if score, ok := result.Scores["self-harm/instructions"]; ok && score >= env.Config.OpenAI.SelfHarmThreshold {
			flagged = append(flagged, FlaggedCategory{
				Category: "self-harm/instructions",
				Score:    score,
			})
		}
	}

	return flagged
}

func IsTextFlagged(ctx context.Context, text string) (bool, []FlaggedCategory) {
	if !env.IsOpenAIModerationEnabled() || text == "" {
		return false, nil
	}

	response, err := CallOpenAIModeration(ctx, text, nil)
	if err != nil {
		return false, nil
	}

	flagged := CheckThresholds(response)
	return len(flagged) > 0, flagged
}

func IsImageFlagged(ctx context.Context, imageData []byte, contentType string) (bool, []FlaggedCategory) {
	if !env.IsOpenAIModerationEnabled() || len(imageData) == 0 {
		return false, nil
	}

	images := []ImageData{{Content: imageData, ContentType: contentType}}
	response, err := CallOpenAIModeration(ctx, "", images)
	if err != nil {
		return false, nil
	}

	flagged := CheckThresholds(response)
	return len(flagged) > 0, flagged
}
