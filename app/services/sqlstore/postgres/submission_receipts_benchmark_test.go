package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func BenchmarkSubmissionReceiptsHTTP(b *testing.B) {
	for _, kind := range []string{"post", "comment", "comment edit", "file"} {
		for _, mode := range []string{"create", "replay"} {
			b.Run(kind+"/"+mode, func(b *testing.B) {
				f := newPostWorkflow(b)
				f.user.Role = enum.RoleAdministrator
				post := &cmd.AddNewPost{Title: "Receipt benchmark owner", Description: "Discussion owner"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					b.Fatal(err)
				}
				comment := &cmd.CreateComment{PostNumber: post.Result.Number, Content: "Existing comment", SubmissionID: "owner"}
				if err := bus.Dispatch(f.ctx, comment); err != nil {
					b.Fatal(err)
				}

				body := map[string]any{"submissionId": "RECEIPT-ID", "content": "Benchmark comment"}
				handler := api.CreateDiscussionComment()
				params := web.StringMap{"number": fmt.Sprint(post.Result.Number)}
				switch kind {
				case "post":
					handler = api.CreatePost()
					body["title"] = "Benchmark complete submission RECEIPT-ID"
					body["description"] = strings.Repeat("A detailed suggestion for a complete operation. ", 6)
				case "comment edit":
					handler = api.EditDiscussionComment()
					params = web.StringMap{"id": fmt.Sprint(comment.Result.ID)}
				case "file":
					handler = handlers.UploadFile()
					body["name"] = "Benchmark image"
					body["uploadType"] = "file"
					body["file"] = pngAttachment(b, 4)
				}

				encoded, err := json.Marshal(body)
				if err != nil {
					b.Fatal(err)
				}
				template := string(encoded)
				request := func(id string) {
					payload := strings.ReplaceAll(template, "RECEIPT-ID", id)
					response, err := f.requestWithParams(handler, http.MethodPost, "/api/submission", payload, params)
					if err != nil || response.Code != http.StatusOK {
						b.Fatalf("status=%d error=%v body=%s", response.Code, err, response.Body)
					}
				}
				request("warm-up")

				b.ReportAllocs()
				b.ResetTimer()
				for index := 0; index < b.N; index++ {
					id := "warm-up"
					if mode == "create" {
						id = fmt.Sprintf("operation-%d", index)
					}
					request(id)
				}
				b.StopTimer()
			})
		}
	}
}
