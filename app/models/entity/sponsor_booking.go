package entity

import "time"

type SponsorBooking struct {
	PlacementID string `json:"placementId"`
	Share       int    `json:"share"`
}

type SponsorConflicts struct {
	Campaign []string `json:"campaign"`
	Creative []string `json:"creative"`
	Bookings []string `json:"bookings"`
	Image    bool     `json:"image"`
}

func (c SponsorConflicts) Any() bool {
	return len(c.Campaign)+len(c.Creative)+len(c.Bookings) > 0 || c.Image
}

type SponsorCampaign struct {
	ID            int              `json:"id"`
	Revision      int              `json:"revision"`
	Name          string           `json:"name"`
	Advertiser    string           `json:"advertiser"`
	Category      string           `json:"category"`
	Exclusive     bool             `json:"exclusive"`
	State         string           `json:"state"`
	StartAt       time.Time        `json:"startAt"`
	EndAt         time.Time        `json:"endAt"`
	ConfirmBy     *time.Time       `json:"confirmBy"`
	Languages     []string         `json:"languages"`
	Countries     []string         `json:"countries"`
	PageTypes     []string         `json:"pageTypes"`
	Bookings      []SponsorBooking `json:"bookings"`
	AmountMinor   int64            `json:"amountMinor"`
	Currency      string           `json:"currency"`
	PaymentStatus string           `json:"paymentStatus"`
	Notes         string           `json:"notes"`
}

type SponsorCreative struct {
	ID           int        `json:"id"`
	CampaignID   int        `json:"campaignId"`
	Revision     int        `json:"revision"`
	State        string     `json:"state"`
	ReviewReason string     `json:"reviewReason"`
	Language     string     `json:"language"`
	Device       string     `json:"device"`
	StartAt      *time.Time `json:"startAt"`
	EndAt        *time.Time `json:"endAt"`
	SponsorArtwork
}

type SponsorArtwork struct {
	Framed       bool              `json:"framed"`
	Headline     string            `json:"headline"`
	Description  string            `json:"description"`
	LogoKey      string            `json:"logoKey"`
	ImageKey     string            `json:"imageKey"`
	BannerCrop   *SponsorImageCrop `json:"bannerCrop,omitempty"`
	CallToAction string            `json:"callToAction"`
	Destination  string            `json:"destination"`
	OfferCode    string            `json:"offerCode"`
	OfferTerms   string            `json:"offerTerms"`
	OfferExpires *time.Time        `json:"offerExpires"`
}

type SponsorImageCrop struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

type SponsorPlacement struct {
	AdSenseSlotID string `json:"adsenseSlotId"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	PageType      string `json:"pageType"`
	Device        string `json:"device"`
	Enabled       bool   `json:"enabled"`
	Position      string `json:"position"`
	Every         int    `json:"every"`
	Empty         string `json:"empty"`
}

var SponsorPlacements = []SponsorPlacement{
	{ID: "strip_desktop", Name: "Community strip on desktop", PageType: "all", Device: "desktop", Position: "navigation"},
	{ID: "strip_mobile", Name: "Community strip on mobile", PageType: "all", Device: "mobile", Position: "navigation"},
	{ID: "home_desktop", Name: "Home sidebar", PageType: "home", Device: "desktop", Position: "sidebar"},
	{ID: "feed_desktop", Name: "Feed on desktop", PageType: "home", Device: "desktop", Position: "feed", Every: 8},
	{ID: "feed_mobile", Name: "Feed on mobile", PageType: "home", Device: "mobile", Position: "feed", Every: 8},
	{ID: "post_desktop", Name: "Post panel on desktop", PageType: "post", Device: "desktop", Position: "before"},
	{ID: "post_mobile", Name: "Post panel on mobile", PageType: "post", Device: "mobile", Position: "before"},
	{ID: "page_desktop", Name: "Page panel on desktop", PageType: "page", Device: "desktop", Position: "after"},
	{ID: "page_mobile", Name: "Page panel on mobile", PageType: "page", Device: "mobile", Position: "after"},
}

type SponsorContext struct {
	PageType string `json:"pageType"`
	ID       int    `json:"id"`
	Language string `json:"language"`
	Device   string `json:"device"`
	Country  string `json:"-"`
}

type SponsorOpportunity struct {
	InstanceID  string `json:"instanceId"`
	PlacementID string `json:"placementId"`
}

type SponsorSelection struct {
	ClickURL   string           `json:"clickUrl,omitempty"`
	Kind       string           `json:"kind"`
	Advertiser string           `json:"advertiser,omitempty"`
	Placement  SponsorPlacement `json:"placement"`
	Creative   *SponsorArtwork  `json:"creative,omitempty"`
	ExpiresAt  *time.Time       `json:"expiresAt,omitempty"`
}

type SponsorAllocation struct {
	Clicks      int64  `json:"clicks" db:"clicks"`
	Day         string `json:"day" db:"day"`
	CampaignID  int    `json:"campaignId" db:"campaign_id"`
	PlacementID string `json:"placementId" db:"placement_id"`
	CreativeID  int    `json:"creativeId" db:"creative_id"`
	Eligible    int64  `json:"eligible" db:"eligible"`
	Allocated   int64  `json:"allocated" db:"allocated"`
	Expected    int64  `json:"expectedHundredths" db:"expected"`
}

type SponsorChange struct {
	CampaignID int       `json:"campaignId" db:"campaign_id"`
	State      string    `json:"state" db:"state"`
	At         time.Time `json:"at" db:"at"`
}

type SponsorExclusion struct {
	PageType string `json:"pageType" db:"page_type"`
	ID       int    `json:"id" db:"content_id"`
	Reason   string `json:"reason" db:"reason"`
}
