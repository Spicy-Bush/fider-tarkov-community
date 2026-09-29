package entity

import "time"

type PageEditSession struct {
	PageID       int          `json:"pageId"`
	State        []byte       `json:"state"`
	StateVector  []byte       `json:"stateVector"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	LegacyDrafts []*PageDraft `json:"legacyDrafts"`
}

type PageEditSync struct {
	Update      []byte    `json:"update"`
	StateVector []byte    `json:"stateVector"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
