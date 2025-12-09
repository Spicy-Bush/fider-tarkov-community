package tasks

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webhook"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

func NotifyAboutNewReport(reportID int, reportedType enum.ReportType, reportedID int, reason string) worker.Task {
	return describe("Notify about new report", func(c *worker.Context) error {
		tenant := c.Tenant()
		baseURL, logoURL := web.BaseURL(c), web.LogoURL(c)

		webhookProps := webhook.Props{
			"reportId":     reportID,
			"reportedType": reportedType.String(),
			"reportedId":   reportedID,
			"reason":       reason,
		}
		webhookProps.SetTenant(tenant, "tenant", baseURL, logoURL)

		err := bus.Dispatch(c, &cmd.TriggerWebhooks{
			Type:  enum.WebhookNewReport,
			Props: webhookProps,
		})
		if err != nil {
			return c.Failure(err)
		}

		return nil
	})
}
