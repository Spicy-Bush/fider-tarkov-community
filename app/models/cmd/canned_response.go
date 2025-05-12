package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

// CreateCannedResponse is used to create a new canned response
type CreateCannedResponse struct {
	Type     string
	Title    string
	Content  string
	Duration string
	Result   *entity.CannedResponse
}

// UpdateCannedResponse is used to update an existing canned response
type UpdateCannedResponse struct {
	ID       int
	Type     string
	Title    string
	Content  string
	Duration string
	IsActive bool
	Result   *entity.CannedResponse
}

// DeleteCannedResponse is used to delete a canned response
type DeleteCannedResponse struct {
	ID int
}
