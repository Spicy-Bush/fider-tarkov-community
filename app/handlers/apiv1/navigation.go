package apiv1

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func SaveNavigationLinks() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.SaveNavigationLinks)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			saveCmd := &cmd.SaveNavigationLinks{
				Links: make([]struct {
					Title        string `json:"title"`
					URL          string `json:"url"`
					DisplayOrder int    `json:"displayOrder"`
					Location     string `json:"location"`
				}, len(action.Links)),
			}

			for i, link := range action.Links {
				saveCmd.Links[i].Title = link.Title
				saveCmd.Links[i].URL = link.URL
				saveCmd.Links[i].DisplayOrder = link.DisplayOrder
				saveCmd.Links[i].Location = link.Location
			}

			if err := bus.Dispatch(c, saveCmd); err != nil {
				return c.Failure(err)
			}

			web.InvalidateNavigationLinksCache()
			return c.Ok(nil)
		})
	}
}
