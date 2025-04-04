package entity

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type BillingState struct {
	Status             enum.BillingStatus `json:"status"`
	PlanID             string             `json:"planID"`
	SubscriptionID     string             `json:"subscriptionID"`
	TrialEndsAt        *time.Time         `json:"trialEndsAt"`
	SubscriptionEndsAt *time.Time         `json:"subscriptionEndsAt"`
}

type BillingSubscription struct {
	UpdateURL          string                    `json:"updateURL"`
	CancelURL          string                    `json:"cancelURL"`
	PaymentInformation BillingPaymentInformation `json:"paymentInformation"`
	LastPayment        BillingPayment            `json:"lastPayment"`
	NextPayment        BillingPayment            `json:"nextPayment"`
}

type BillingPaymentInformation struct {
	PaymentMethod  string `json:"paymentMethod"`
	CardType       string `json:"cardType"`
	LastFourDigits string `json:"lastFourDigits"`
	ExpiryDate     string `json:"expiryDate"`
}

type BillingPayment struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Date     string  `json:"date"`
}
