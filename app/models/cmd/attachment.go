package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type UploadImage struct {
	Image  *dto.ImageUpload
	Folder string
}
