package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
)

func TestManageBillingHandler_RedirectWhenUsingCNAME(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	code, response := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		WithURL("https://feedback.demo.com/admin/billing").
		Execute(handlers.ManageBilling())

	Expect(code).Equals(http.StatusTemporaryRedirect)
	Expect(response.Header().Get("Location")).Equals("https://demo.test.fider.io/admin/billing")
}

func TestManageBillingHandler_ReturnsCorrectBillingInformation(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetBillingState) error {
		trialEndsAt := time.Date(2021, time.February, 2, 4, 2, 2, 0, time.UTC)
		q.Result = &entity.BillingState{
			Status:         enum.BillingActive,
			PlanID:         "PLAN-123",
			SubscriptionID: "SUB-123",
			TrialEndsAt:    &trialEndsAt,
		}
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetBillingSubscription) error {
		Expect(q.SubscriptionID).Equals("SUB-123")

		q.Result = &entity.BillingSubscription{
			UpdateURL: "https://sandbox-subscription-management.paddle.com/subscription/SUB-123/hash/1111/update",
			CancelURL: "https://sandbox-subscription-management.paddle.com/subscription/SUB-123/hash/1111/cancel",
			PaymentInformation: entity.BillingPaymentInformation{
				PaymentMethod:  "card",
				CardType:       "visa",
				LastFourDigits: "1111",
				ExpiryDate:     "10/2031",
			},
			LastPayment: entity.BillingPayment{
				Amount:   float64(30),
				Currency: "USD",
				Date:     "2021-11-09",
			},
		}
		return nil
	})

	server := mock.NewServer()
	code, page := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		WithURL("https://demo.test.fider.io/admin/billing").
		ExecuteAsPage(handlers.ManageBilling())

	Expect(code).Equals(http.StatusOK)
	Expect(page.Data).ContainsProps(dto.Props{
		"status":             float64(2),
		"trialEndsAt":        "2021-02-02T04:02:02Z",
		"subscriptionEndsAt": nil,
	})

	Expect(page.Data["subscription"]).ContainsProps(dto.Props{
		"updateURL": "https://sandbox-subscription-management.paddle.com/subscription/SUB-123/hash/1111/update",
		"cancelURL": "https://sandbox-subscription-management.paddle.com/subscription/SUB-123/hash/1111/cancel",
	})

	ExpectHandler(&query.GetBillingState{}).CalledOnce()
	ExpectHandler(&query.GetBillingSubscription{}).CalledOnce()
}

func TestGenerateCheckoutLinkHandler(t *testing.T) {
	RegisterT(t)

	env.Config.Paddle.VendorID = "123"
	env.Config.Paddle.VendorAuthCode = "456"
	env.Config.Paddle.MonthlyPlanID = "PLAN_M"
	env.Config.Paddle.YearlyPlanID = "PLAN_Y"

	bus.AddHandler(func(ctx context.Context, c *cmd.GenerateCheckoutLink) error {
		c.URL = "https://paddle.com/fake-checkout-url"
		return nil
	})

	server := mock.NewServer()
	code, json := server.
		WithURL("http://demo.test.fider.io/_api/billing/checkout-link").
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePostAsJSON(handlers.GenerateCheckoutLink(), "{ \"planID\": \"PLAN_M\" }")

	Expect(code).Equals(http.StatusOK)
	Expect(json.String("url")).Equals("https://paddle.com/fake-checkout-url")
	ExpectHandler(&cmd.GenerateCheckoutLink{}).CalledOnce()
}
