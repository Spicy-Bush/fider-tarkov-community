package postgres

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestPostSearchBindsPagination(t *testing.T) {
	for _, test := range []struct {
		name   string
		view   string
		query  string
		limit  string
		offset string
		valid  bool
	}{
		{name: "numeric", view: "newest", limit: "2", offset: "1", valid: true},
		{name: "recently updated", view: "recently-updated", limit: "2", offset: "1", valid: true},
		{name: "recently updated search", view: "recently-updated", query: "Parameter", limit: "2", offset: "1", valid: true},
		{name: "malicious limit", view: "newest", limit: "2); SELECT 42 --", offset: "0"},
		{name: "malicious offset", view: "newest", limit: "2", offset: "0); SELECT 42 --"},
	} {
		t.Run(test.name, func(t *testing.T) {
			trx, err := dbx.BeginTx(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			defer trx.MustRollback()

			ids := make([]int, 5)
			for i := range ids {
				if err := trx.Get(&ids[i], `INSERT INTO posts
					(tenant_id, user_id, title, slug, description, status, created_at)
					VALUES (1, 1, 'Parameter boundary', 'parameter-boundary-' || $1::integer, 'Content', 0, NOW()) RETURNING id`, i); err != nil {
					t.Fatal(err)
				}
			}

			search := query.SearchPosts{
				IDs:    ids,
				View:   test.view,
				Query:  test.query,
				Date:   "1d",
				Limit:  test.limit,
				Offset: test.offset,
			}

			sql, args := buildSearchQuery(search, &entity.Tenant{ID: 1}, &entity.User{
				ID:     1,
				Role:   enum.RoleAdministrator,
				Status: enum.UserActive,
			})

			if args[len(args)-2] != test.limit || args[len(args)-1] != test.offset {
				t.Fatal("pagination was not passed as query values")
			}
			if strings.Contains(sql, "SELECT 42") {
				t.Fatal("pagination input became executable SQL")
			}

			var records []*dbPost
			err = trx.Select(&records, sql, args...)
			if !test.valid {
				if err == nil {
					t.Fatal("non-numeric pagination was accepted")
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			got := make([]int, len(records))
			for i, record := range records {
				got[i] = record.ID
			}

			if want := []int{ids[3], ids[2]}; !slices.Equal(got, want) {
				t.Fatalf("paginated IDs %v; want %v", got, want)
			}
		})
	}
}
