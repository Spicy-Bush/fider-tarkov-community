package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type GetPageEdit struct {
	PageID int
	Result *entity.PageEditSession
}
