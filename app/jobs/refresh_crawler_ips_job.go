package jobs

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crawler"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

type RefreshCrawlerIPsJobHandler struct{}

func (e RefreshCrawlerIPsJobHandler) Schedule() string {
	return "0 0 3 * * *"
}

func (e RefreshCrawlerIPsJobHandler) Run(ctx Context) error {
	log.Debug(ctx, "Refreshing crawler IP ranges")

	if crawler.DefaultVerifier == nil {
		log.Warn(ctx, "Crawler verifier not initialised, skipping refresh")
		return nil
	}

	err := crawler.DefaultVerifier.Refresh(ctx)
	if err != nil {
		log.Errorf(ctx, "Failed to refresh some crawler IP ranges: @{Error}", dto.Props{
			"Error": err.Error(),
		})
	}

	return nil
}
