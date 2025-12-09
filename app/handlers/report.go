package handlers

import (
	"net/http"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func ManageReportsPage() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageReports.page",
			Title: "Reports - Site Settings",
		})
	}
}

func ReportPost() web.HandlerFunc {
	return func(c *web.Context) error {
		postNumber, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: postNumber}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.NotFound()
		}

		action := new(actions.CreateReport)
		action.ReportedType = "post"
		action.ReportedID = getPost.Result.ID
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createReport := &cmd.CreateReport{
				ReportedType: enum.ReportTypePost,
				ReportedID:   getPost.Result.ID,
				Reason:       action.Reason,
				Details:      action.Details,
			}
			if err := bus.Dispatch(c, createReport); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutNewReport(createReport.Result, enum.ReportTypePost, getPost.Result.ID, action.Reason))

			sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgReportNew, sse.ReportEventPayload{
				ReportID:     createReport.Result,
				ReportedType: "post",
				ReportedID:   getPost.Result.ID,
				Reason:       action.Reason,
			})

			return c.Ok(web.Map{"id": createReport.Result})
		})
	}
}

func ReportComment() web.HandlerFunc {
	return func(c *web.Context) error {
		postNumber, err := c.ParamAsInt("number")
		if err != nil {
			return c.NotFound()
		}

		commentID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getPost := &query.GetPostByNumber{Number: postNumber}
		if err := bus.Dispatch(c, getPost); err != nil {
			return c.NotFound()
		}

		getComment := &query.GetCommentByID{CommentID: commentID}
		if err := bus.Dispatch(c, getComment); err != nil {
			return c.NotFound()
		}

		action := new(actions.CreateReport)
		action.ReportedType = "comment"
		action.ReportedID = commentID
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createReport := &cmd.CreateReport{
				ReportedType: enum.ReportTypeComment,
				ReportedID:   commentID,
				Reason:       action.Reason,
				Details:      action.Details,
			}
			if err := bus.Dispatch(c, createReport); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyAboutNewReport(createReport.Result, enum.ReportTypeComment, commentID, action.Reason))

			sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgReportNew, sse.ReportEventPayload{
				ReportID:     createReport.Result,
				ReportedType: "comment",
				ReportedID:   commentID,
				Reason:       action.Reason,
			})

			return c.Ok(web.Map{"id": createReport.Result})
		})
	}
}

func ListReports() web.HandlerFunc {
	return func(c *web.Context) error {
		page, _ := c.QueryParamAsInt("page")
		perPage, _ := c.QueryParamAsInt("perPage")
		statusParam := c.QueryParam("status")
		typeParam := c.QueryParam("type")
		reasonParam := c.QueryParam("reason")

		if page < 1 {
			page = 1
		}
		if perPage < 1 || perPage > 50 {
			perPage = 20
		}

		var statuses []enum.ReportStatus
		if statusParam != "" {
			statusParts := strings.Split(statusParam, ",")
			for _, sp := range statusParts {
				sp = strings.TrimSpace(sp)
				if sp != "" {
					var status enum.ReportStatus
					_ = status.UnmarshalText([]byte(sp))
					if status != 0 {
						statuses = append(statuses, status)
					}
				}
			}
		}

		var reportType enum.ReportType
		if typeParam != "" {
			_ = reportType.UnmarshalText([]byte(typeParam))
		}

		listReports := &query.ListReports{
			Status:  statuses,
			Type:    reportType,
			Reason:  reasonParam,
			Page:    page,
			PerPage: perPage,
		}

		if err := bus.Dispatch(c, listReports); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"reports": listReports.Result,
			"total":   listReports.Total,
			"page":    page,
			"perPage": perPage,
			"viewers": sse.GetHub().GetAllActiveViewers(c.Tenant().ID),
		})
	}
}

func GetReport() web.HandlerFunc {
	return func(c *web.Context) error {
		reportID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getReport := &query.GetReportByID{ReportID: reportID}
		if err := bus.Dispatch(c, getReport); err != nil {
			return c.NotFound()
		}

		return c.Ok(getReport.Result)
	}
}

// returns a report with its reported content (post or comment)
func GetReportDetails() web.HandlerFunc {
	return func(c *web.Context) error {
		reportID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getReport := &query.GetReportByID{ReportID: reportID}
		if err := bus.Dispatch(c, getReport); err != nil {
			return c.NotFound()
		}

		report := getReport.Result
		result := web.Map{
			"report": report,
		}

		switch report.ReportedType {
		case enum.ReportTypePost:
			getPost := &query.GetPostByID{PostID: report.ReportedID}
			if err := bus.Dispatch(c, getPost); err == nil {
				result["post"] = getPost.Result
			}
		case enum.ReportTypeComment:
			getComment := &query.GetCommentByID{CommentID: report.ReportedID}
			if err := bus.Dispatch(c, getComment); err == nil {
				result["comment"] = getComment.Result
			}
			if report.PostNumber > 0 {
				getPost := &query.GetPostByNumber{Number: report.PostNumber}
				if err := bus.Dispatch(c, getPost); err == nil {
					result["post"] = getPost.Result
				}
			}
		}

		return c.Ok(result)
	}
}

func AssignReport() web.HandlerFunc {
	return func(c *web.Context) error {
		reportID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.AssignReport{ReportID: reportID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			assignReport := &cmd.AssignReport{
				ReportID:   reportID,
				AssignToID: c.User().ID,
			}
			if err := bus.Dispatch(c, assignReport); err != nil {
				return c.Failure(err)
			}

			sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgReportAssigned, sse.ReportEventPayload{
				ReportID: reportID,
				AssignedTo: &sse.ClientInfo{
					UserID:   c.User().ID,
					UserName: c.User().Name,
				},
			})

			return c.Ok(web.Map{})
		})
	}
}

func UnassignReport() web.HandlerFunc {
	return func(c *web.Context) error {
		reportID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getReport := &query.GetReportByID{ReportID: reportID}
		if err := bus.Dispatch(c, getReport); err != nil {
			return c.NotFound()
		}

		return c.WithTransaction(func() error {
			unassignReport := &cmd.UnassignReport{ReportID: reportID}
			if err := bus.Dispatch(c, unassignReport); err != nil {
				return c.Failure(err)
			}

			sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgReportUnassigned, sse.ReportEventPayload{
				ReportID: reportID,
			})

			return c.Ok(web.Map{})
		})
	}
}

func ResolveReport() web.HandlerFunc {
	return func(c *web.Context) error {
		reportID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := new(actions.ResolveReport)
		action.ReportID = reportID
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		var status enum.ReportStatus
		_ = status.UnmarshalText([]byte(action.Status))

		return c.WithTransaction(func() error {
			resolveReport := &cmd.ResolveReport{
				ReportID:       reportID,
				Status:         status,
				ResolutionNote: action.ResolutionNote,
			}
			if err := bus.Dispatch(c, resolveReport); err != nil {
				return c.Failure(err)
			}

			sse.GetHub().BroadcastToTenant(c.Tenant().ID, sse.MsgReportResolved, sse.ReportEventPayload{
				ReportID: reportID,
				Status:   action.Status,
			})

			return c.Ok(web.Map{})
		})
	}
}

func GetReportReasons() web.HandlerFunc {
	return func(c *web.Context) error {
		getReasons := &query.GetReportReasons{}
		if err := bus.Dispatch(c, getReasons); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getReasons.Result)
	}
}

func ReportHeartbeat() web.HandlerFunc {
	return func(c *web.Context) error {
		reportID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getReport := &query.GetReportByID{ReportID: reportID}
		if err := bus.Dispatch(c, getReport); err != nil {
			return c.NotFound()
		}

		sse.GetHub().UpdatePresence(c.Tenant().ID, c.User().ID, c.User().Name, reportID)

		return c.Ok(web.Map{})
	}
}

func StopViewingReport() web.HandlerFunc {
	return func(c *web.Context) error {
		sse.GetHub().UpdatePresence(c.Tenant().ID, c.User().ID, c.User().Name, 0)

		return c.Ok(web.Map{})
	}
}

func ListAllReportReasons() web.HandlerFunc {
	return func(c *web.Context) error {
		listReasons := &query.ListAllReportReasons{}
		if err := bus.Dispatch(c, listReasons); err != nil {
			return c.Failure(err)
		}

		return c.Ok(listReasons.Result)
	}
}

func CreateReportReason() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateReportReason)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createReason := &cmd.CreateReportReason{
				Title:       action.Title,
				Description: action.Description,
			}
			if err := bus.Dispatch(c, createReason); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{"id": createReason.Result})
		})
	}
}

func UpdateReportReason() web.HandlerFunc {
	return func(c *web.Context) error {
		reasonID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid reason ID"})
		}

		action := new(actions.UpdateReportReason)
		action.ID = reasonID
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateReason := &cmd.UpdateReportReason{
				ID:          reasonID,
				Title:       action.Title,
				Description: action.Description,
				IsActive:    action.IsActive,
			}
			if err := bus.Dispatch(c, updateReason); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func DeleteReportReason() web.HandlerFunc {
	return func(c *web.Context) error {
		reasonID, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid reason ID"})
		}

		action := &actions.DeleteReportReason{ID: reasonID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			deleteReason := &cmd.DeleteReportReason{ID: reasonID}
			if err := bus.Dispatch(c, deleteReason); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func ReorderReportReasons() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.ReorderReportReasons)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			reorder := &cmd.ReorderReportReasons{
				IDs: action.IDs,
			}
			if err := bus.Dispatch(c, reorder); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}
