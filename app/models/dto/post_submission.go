package dto

type PostSubmissionReceipt struct {
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Slug   string `json:"slug"`
}