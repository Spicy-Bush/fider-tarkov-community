package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type GetPushSubscriptionsByUser struct {
	UserID int
	Result []*entity.PushSubscription
}

type GetPushSubscriptionsByUsers struct {
	UserIDs []int
	Result  []*entity.PushSubscription
}

type GetAllPushSubscriptions struct {
	Result []*entity.PushSubscription
}

type HasPushSubscription struct {
	UserID int
	Result bool
}

