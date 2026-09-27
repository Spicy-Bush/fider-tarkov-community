package web_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestBindToRejectsInvalidJSONBeforeExecutingAction(t *testing.T) {
	for _, body := range []string{
		`{"status":`,
		`{"status":"unknown"}`,
		`{"status":1}`,
		`{"status":{}}`,
		`[]`,
	} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/posts/1/response", strings.NewReader(body))
			request.Header.Set("Content-Type", web.JSONContentType)
			response := httptest.NewRecorder()
			ctx, err := web.NewContext(web.New(), request, response, nil)
			if err != nil {
				t.Fatal(err)
			}

			status := enum.PostStarted
			action := &actions.SetResponse{Status: &status}
			if err := ctx.HandleValidation(ctx.BindTo(action)); err != nil {
				t.Fatal(err)
			}

			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid input returned %d: %s", response.Code, response.Body.String())
			}
			if action.Status == nil || *action.Status != enum.PostStarted {
				t.Fatal("invalid input changed the status")
			}
		})
	}
}

func TestBinderPreservesInvalidTargetErrors(t *testing.T) {
	ctx := newBodyContext(http.MethodPost, nil, `{}`, web.JSONContentType)
	err := web.NewDefaultBinder().Bind(actions.SetResponse{}, ctx)

	var invalidTarget *json.InvalidUnmarshalError
	if !errors.As(err, &invalidTarget) || errors.Is(err, web.ErrInvalidRequestBody) {
		t.Fatalf("invalid binding target was classified as client input: %v", err)
	}
}
