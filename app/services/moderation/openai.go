package moderation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
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
	Scores     map[string]*float64 `json:"category_scores"`
	InputTypes map[string][]string `json:"category_applied_input_types,omitempty"`
}

type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

func CallOpenAIModeration(ctx context.Context, text string, images []ImageData) (*ModerationResponse, error) {
	if env.Config.OpenAI.APIKey == "" {
		return nil, errors.New("OpenAI API key not configured")
	}

	if text == "" && len(images) == 0 {
		return nil, errors.New("no content to moderate")
	}

	inputs := make([]ModerationInput, 0, 1+len(images))

	if text != "" {
		inputs = append(inputs, ModerationInput{Type: "text", Text: text})
	}

	for _, img := range images {
		inputs = append(inputs, ModerationInput{
			Type:     "image_url",
			ImageURL: &ImageURLInput{URL: "data:" + img.ContentType + ";base64," + base64.StdEncoding.EncodeToString(img.Content)},
		})
	}

	return callSingleModeration(ctx, inputs)
}

// diagnostics
type ProviderError struct {
	Status     int
	Type       string
	Headers    map[string]string
	Cause      error
	Code       string
	RequestID  string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	headers, _ := json.Marshal(e.Headers)
	return fmt.Sprintf("moderation unavailable: status=%d type=%s code=%s request_id=%s headers=%s", e.Status, e.Type, e.Code, e.RequestID, headers)
}

func (e *ProviderError) Unwrap() error { return e.Cause }

func callSingleModeration(ctx context.Context, inputs []ModerationInput) (*ModerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(ModerationRequest{Model: "omni-moderation-latest", Input: inputs})

	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request := &cmd.HTTPRequest{
		URL: OpenAIModerationURL, Method: "POST", Body: bytes.NewReader(payload),
		Headers: map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + env.Config.OpenAI.APIKey},
	}

	if err := bus.Dispatch(requestCtx, request); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, &ProviderError{Code: "transport_error", Retryable: true, Cause: err}
	}

	if request.ResponseStatusCode != 200 {
		var body struct {
			Error struct {
				Code string `json:"code"`
				Type string `json:"type"`
			} `json:"error"`
		}

		_ = json.Unmarshal(request.ResponseBody, &body)
		failure := &ProviderError{
			Status: request.ResponseStatusCode, Type: safeDiagnostic(body.Error.Type), Headers: diagnosticHeaders(request.ResponseHeader), Code: safeDiagnostic(body.Error.Code),
			RequestID: safeDiagnostic(request.ResponseHeader.Get("x-request-id")),
			Retryable: request.ResponseStatusCode == 429 || request.ResponseStatusCode >= 500 || request.ResponseStatusCode == 408,
		}

		if request.ResponseStatusCode == 429 || request.ResponseStatusCode == 503 {
			cooldown := retryDelay(request.ResponseHeader, time.Now())

			if body.Error.Code == "insufficient_quota" && cooldown < time.Hour {
				cooldown = time.Hour
			}

			failure.RetryAfter = cooldown
		}

		return nil, failure
	}

	var response ModerationResponse

	if err := json.Unmarshal(request.ResponseBody, &response); err != nil || len(response.Results) != 1 {
		return nil, &ProviderError{Code: "invalid_response", Retryable: true}
	}

	for _, category := range []string{"sexual", "sexual/minors", "self-harm", "self-harm/intent", "self-harm/instructions"} {
		score, exists := response.Results[0].Scores[category]

		if !exists || score == nil || math.IsNaN(*score) || math.IsInf(*score, 0) || *score < 0 || *score > 1 {
			return nil, &ProviderError{Code: "incomplete_response", Retryable: true}
		}
	}

	return &response, nil
}

func diagnosticHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)

	for name, values := range headers {
		lower := strings.ToLower(name)

		if lower != "date" && lower != "retry-after" && !strings.HasPrefix(lower, "x-ratelimit-") {
			continue
		}

		if len(result) == 20 {
			break
		}

		value := strings.Join(values, ", ")

		if len(value) > 256 {
			value = value[:256]
		}

		result[lower] = strings.Map(func(r rune) rune {
			if r < 32 || r > 126 {
				return -1
			}

			return r
		}, value)
	}

	return result
}

func safeDiagnostic(value string) string {
	if len(value) > 128 {
		return "unrecognized"
	}

	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return "unrecognized"
		}
	}

	return value
}

func retryDelay(headers http.Header, now time.Time) time.Duration {
	delay := 5 * time.Second

	if seconds, err := strconv.ParseFloat(headers.Get("Retry-After"), 64); err == nil && seconds > 0 && !math.IsInf(seconds, 0) && !math.IsNaN(seconds) {
		delay = time.Duration(math.Min(seconds, 86400) * float64(time.Second))
	} else if date, err := http.ParseTime(headers.Get("Retry-After")); err == nil && date.After(now) {
		delay = date.Sub(now)
	}

	for _, key := range []string{"x-ratelimit-reset-requests", "x-ratelimit-reset-tokens"} {
		if value, err := time.ParseDuration(headers.Get(key)); err == nil && value > delay {
			delay = value
		}
	}

	return min(delay, 24*time.Hour)
}

func CheckThresholds(response *ModerationResponse) []cmd.ModerationFinding {
	flagged := []cmd.ModerationFinding{}

	if len(response.Results) == 0 {
		return flagged
	}

	for _, result := range response.Results {
		if score, ok := result.Scores["sexual"]; ok && score != nil && *score >= env.Config.OpenAI.SexualThreshold {
			flagged = append(flagged, cmd.ModerationFinding{
				Category: "sexual",
				Score:    *score,
			})
		}

		if score, ok := result.Scores["sexual/minors"]; ok && score != nil && *score >= env.Config.OpenAI.SexualThreshold {
			flagged = append(flagged, cmd.ModerationFinding{
				Category: "sexual/minors",
				Score:    *score,
			})
		}

		if score, ok := result.Scores["self-harm"]; ok && score != nil && *score >= env.Config.OpenAI.SelfHarmThreshold {
			flagged = append(flagged, cmd.ModerationFinding{
				Category: "self-harm",
				Score:    *score,
			})
		}

		if score, ok := result.Scores["self-harm/intent"]; ok && score != nil && *score >= env.Config.OpenAI.SelfHarmThreshold {
			flagged = append(flagged, cmd.ModerationFinding{
				Category: "self-harm/intent",
				Score:    *score,
			})
		}

		if score, ok := result.Scores["self-harm/instructions"]; ok && score != nil && *score >= env.Config.OpenAI.SelfHarmThreshold {
			flagged = append(flagged, cmd.ModerationFinding{
				Category: "self-harm/instructions",
				Score:    *score,
			})
		}
	}

	return flagged
}
