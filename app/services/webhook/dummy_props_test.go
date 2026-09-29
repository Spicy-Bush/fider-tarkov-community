package webhook

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestWebhookPreviewPropertiesStayWithinTenant(t *testing.T) {
	var workers sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan string, 8)

	for tenantID := 1; tenantID <= 8; tenantID++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			email := fmt.Sprintf("admin-%d@example.test", tenantID)
			tenant := &entity.Tenant{ID: tenantID}
			user := &entity.User{ID: tenantID, Email: email, Tenant: tenant}
			requestURL, err := url.Parse(fmt.Sprintf("https://tenant-%d.example.test", tenantID))
			if err != nil {
				failures <- err.Error()
				return
			}

			ctx := context.WithValue(context.Background(), app.TenantCtxKey, tenant)
			ctx = context.WithValue(ctx, app.UserCtxKey, user)
			ctx = context.WithValue(ctx, app.RequestCtxKey, web.Request{URL: requestURL})
			<-start

			for attempt := 0; attempt < 100; attempt++ {
				props := dummyTriggerProps(ctx, enum.WebhookChangeStatus)
				if props["post_response_author_email"] != email {
					failures <- fmt.Sprintf("tenant %d received another author's email: %v", tenantID, props["post_response_author_email"])
					return
				}
			}
		}()
	}

	close(start)
	workers.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}
