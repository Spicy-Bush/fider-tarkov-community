package cmd

type SavePushSubscription struct {
	Endpoint  string
	KeyP256dh string
	KeyAuth   string
}

type DeletePushSubscription struct {
	Endpoint string
}

type DeleteAllPushSubscriptions struct{}

type DeletePushSubscriptionByEndpoint struct {
	Endpoint string
}

type SendPushNotification struct {
	UserID int
	Title  string
	Body   string
	Icon   string
	URL    string
	Tag    string
}

