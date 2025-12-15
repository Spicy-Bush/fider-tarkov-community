package tasks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/moderation"
)

func ModerateNewContent(contentType string, contentID int, text string, imageBlobKeys []string) worker.Task {
	return describe("Moderate content via OpenAI", func(c *worker.Context) error {
		if !env.IsOpenAIModerationEnabled() {
			return nil
		}

		if text == "" && len(imageBlobKeys) == 0 {
			return nil
		}

		var images []moderation.ImageData
		for _, blobKey := range imageBlobKeys {
			getBlob := &query.GetBlobByKey{Key: blobKey}
			if err := bus.Dispatch(c, getBlob); err != nil {
				log.Warn(c, fmt.Sprintf("Failed to fetch blob for moderation: %s", err.Error()))
				continue
			}
			if getBlob.Result != nil && len(getBlob.Result.Content) > 0 {
				images = append(images, moderation.ImageData{
					Content:     getBlob.Result.Content,
					ContentType: getBlob.Result.ContentType,
				})
			}
		}

		if text == "" && len(images) == 0 {
			return nil
		}

		response, err := moderation.CallOpenAIModeration(c, text, images)
		if err != nil {
			log.Warn(c, fmt.Sprintf("OpenAI moderation API call failed: %s", err.Error()))
			return nil
		}

		flaggedCategories := moderation.CheckThresholds(response)

		if len(flaggedCategories) == 0 {
			return nil
		}

		categoryNames := make([]string, len(flaggedCategories))
		for i, fc := range flaggedCategories {
			categoryNames[i] = fmt.Sprintf("%s (%.2f)", fc.Category, fc.Score)
		}
		log.Warn(c, fmt.Sprintf("Content flagged by OpenAI moderation: %s %d - categories: %s",
			contentType, contentID, strings.Join(categoryNames, ", ")))

		moderationData, _ := json.Marshal(response)

		setPending := &cmd.SetModerationPending{
			ContentType:    contentType,
			ContentID:      contentID,
			Pending:        true,
			ModerationData: string(moderationData),
		}
		if err := bus.Dispatch(c, setPending); err != nil {
			return c.Failure(err)
		}

		var reportedType enum.ReportType
		if contentType == "post" {
			reportedType = enum.ReportTypePost
		} else {
			reportedType = enum.ReportTypeComment
		}

		createReport := &cmd.CreateReport{
			ReportedType: reportedType,
			ReportedID:   contentID,
			Reason:       "Auto-flagged by AI moderation",
			Details:      fmt.Sprintf("Flagged categories: %s", strings.Join(categoryNames, ", ")),
			ReporterID:   nil,
		}
		if err := bus.Dispatch(c, createReport); err != nil {
			log.Warn(c, fmt.Sprintf("Failed to create auto-report: %s", err.Error()))
		}

		return nil
	})
}

