package tasks

import (
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

func NotifyPageSubscribers(pageID int, updatedByUserID int) worker.Task {
	return describe("Notify page subscribers", func(c *worker.Context) error {
		getSubscribers := &query.GetPageSubscribers{PageID: pageID}
		if err := bus.Dispatch(c, getSubscribers); err != nil {
			return err
		}

		getPage := &query.GetPageByID{ID: pageID}
		if err := bus.Dispatch(c, getPage); err != nil {
			return err
		}

		page := getPage.Result

		for _, subscriber := range getSubscribers.Result {
			if subscriber.ID != updatedByUserID {
				title := fmt.Sprintf("Page updated: %s", page.Title)
				link := fmt.Sprintf("/pages/%s", page.Slug)

				if err := bus.Dispatch(c, &cmd.AddNewNotification{
					User:   subscriber,
					Title:  title,
					Link:   link,
					PageID: pageID,
				}); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func PublishScheduledPages() worker.Task {
	return describe("Publish scheduled pages", func(c *worker.Context) error {
		publishCmd := &cmd.PublishScheduledPages{}
		if err := bus.Dispatch(c, publishCmd); err != nil {
			return err
		}

		if publishCmd.Result > 0 {
			log.Infof(c, "Published scheduled pages", dto.Props{
				"count": publishCmd.Result,
			})
		}

		return nil
	})
}
