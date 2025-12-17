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
					PostID: 0,
				}); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func NotifyPageComment(pageID int, commentID int) worker.Task {
	return describe("Notify page comment to admins", func(c *worker.Context) error {
		getPage := &query.GetPageByID{ID: pageID}
		if err := bus.Dispatch(c, getPage); err != nil {
			return err
		}

		page := getPage.Result

		title := fmt.Sprintf("New comment on page: %s", page.Title)
		link := fmt.Sprintf("/pages/%s#comment-%d", page.Slug, commentID)

		getAllUsers := &query.GetAllUsers{}
		if err := bus.Dispatch(c, getAllUsers); err != nil {
			return err
		}

		for _, user := range getAllUsers.Result {
			if user.IsAdministrator() || user.IsCollaborator() {
				if err := bus.Dispatch(c, &cmd.AddNewNotification{
					User:   user,
					Title:  title,
					Link:   link,
					PostID: 0,
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
