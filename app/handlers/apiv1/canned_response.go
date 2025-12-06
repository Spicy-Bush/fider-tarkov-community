package apiv1

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// ListCannedResponses returns all canned responses of a specific type
func ListCannedResponses() web.HandlerFunc {
	return func(c *web.Context) error {
		responseType := c.Param("type")

		listCannedResponses := &query.ListCannedResponses{
			Type: responseType,
		}

		if err := bus.Dispatch(c, listCannedResponses); err != nil {
			return c.Failure(err)
		}

		// always return an array, even if Result is nil. Helps with JSON serialization in React
		if listCannedResponses.Result == nil {
			listCannedResponses.Result = []*entity.CannedResponse{}
		}

		return c.Ok(listCannedResponses.Result)
	}
}

// CreateCannedResponse creates a new canned response
func CreateCannedResponse() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateCannedResponse)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createCannedResponse := &cmd.CreateCannedResponse{
				Type:     action.Type,
				Title:    action.Title,
				Content:  action.Content,
				Duration: action.Duration,
			}

			if err := bus.Dispatch(c, createCannedResponse); err != nil {
				return c.Failure(err)
			}

			return c.Ok(createCannedResponse.Result)
		})
	}
}

// UpdateCannedResponse updates an existing canned response
func UpdateCannedResponse() web.HandlerFunc {
	return func(c *web.Context) error {
		responseID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{
				"message": "Invalid response ID",
			})
		}

		action := new(actions.UpdateCannedResponse)
		action.ID = responseID
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateCannedResponse := &cmd.UpdateCannedResponse{
				ID:       responseID,
				Type:     action.Type,
				Title:    action.Title,
				Content:  action.Content,
				Duration: action.Duration,
				IsActive: action.IsActive,
			}

			if err := bus.Dispatch(c, updateCannedResponse); err != nil {
				return c.Failure(err)
			}

			return c.Ok(updateCannedResponse.Result)
		})
	}
}

// DeleteCannedResponse deletes a canned response
func DeleteCannedResponse() web.HandlerFunc {
	return func(c *web.Context) error {
		responseID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{
				"message": "Invalid response ID",
			})
		}

		action := &actions.DeleteCannedResponse{
			ID: responseID,
		}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			deleteCannedResponse := &cmd.DeleteCannedResponse{
				ID: responseID,
			}

			if err := bus.Dispatch(c, deleteCannedResponse); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}
