package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

// GetCannedResponseByID gets a single canned response by its ID
type GetCannedResponseByID struct {
	ID     int
	Result *entity.CannedResponse
}

// ListCannedResponses gets all canned responses of a specific type
type ListCannedResponses struct {
	Type   string
	Result []*entity.CannedResponse
}
