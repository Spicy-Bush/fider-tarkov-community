package csv_test

import (
	"bytes"
	gocsv "encoding/csv"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/csv"
)

func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func TestExportPostsToCSV_Empty(t *testing.T) {
	RegisterT(t)

	posts := []*entity.Post{}
	expected, err := os.ReadFile("./testdata/empty.csv")
	Expect(err).IsNil()
	actual, err := csv.FromPosts(posts)
	Expect(err).IsNil()
	Expect(normalizeLineEndings(actual)).Equals(normalizeLineEndings(expected))
}

func TestExportPostsToCSV_OnePost(t *testing.T) {
	RegisterT(t)

	posts := []*entity.Post{
		declinedPost,
	}

	expected, err := os.ReadFile("./testdata/one-post.csv")
	Expect(err).IsNil()
	actual, err := csv.FromPosts(posts)
	Expect(err).IsNil()
	Expect(normalizeLineEndings(actual)).Equals(normalizeLineEndings(expected))
}

func TestExportPostsToCSV_MorePosts(t *testing.T) {
	RegisterT(t)

	posts := []*entity.Post{
		declinedPost,
		openPost,
		duplicatePost,
	}

	expected, err := os.ReadFile("./testdata/more-posts.csv")
	Expect(err).IsNil()
	actual, err := csv.FromPosts(posts)
	Expect(err).IsNil()
	Expect(normalizeLineEndings(actual)).Equals(normalizeLineEndings(expected))
}

func TestExportedTextCannotBecomeSpreadsheetFormulas(t *testing.T) {
	for _, value := range []string{"=1+1", "+1+1", "-1+1", "@SUM(1)", " \t=1+1", "\r\n=1+1"} {
		t.Run(value, func(t *testing.T) {
			post := *declinedPost
			post.Title = value
			post.Description = value
			post.User = &entity.User{Name: value}
			post.Tags = []string{value}
			post.Response = &entity.PostResponse{
				Text: value,
				User: &entity.User{Name: value},
				Original: &entity.OriginalPost{Number: 2, Title: value},
			}

			data, err := csv.FromPosts([]*entity.Post{&post})
			if err != nil {
				t.Fatal(err)
			}
			rows, err := gocsv.NewReader(bytes.NewReader(data)).ReadAll()
			if err != nil || len(rows) != 2 {
				t.Fatalf("invalid CSV: %v", err)
			}

			for _, column := range []int{1, 2, 4, 8, 10, 12, 13} {
				if !strings.HasPrefix(rows[1][column], "'") {
					t.Errorf("column %s contains executable spreadsheet text %q", rows[0][column], rows[1][column])
				}
			}
			if rows[1][5] != "4" {
				t.Fatal("numeric vote count became a text cell")
			}
		})
	}
}

var declinedPost = &entity.Post{
	Number:      10,
	Title:       "Go is fast",
	Description: "Very tiny description",
	CreatedAt:   time.Date(2018, 3, 23, 19, 33, 22, 0, time.UTC),
	User: &entity.User{
		Name: "Faceless",
	},
	VotesCount:    4,
	CommentsCount: 2,
	Status:        enum.PostDeclined,
	Response: &entity.PostResponse{
		Text:        "Nothing we need to do",
		RespondedAt: time.Date(2018, 4, 4, 19, 48, 10, 0, time.UTC),
		User: &entity.User{
			Name: "John Snow",
		},
	},
	Tags: []string{"easy", "ignored"},
}

var openPost = &entity.Post{
	Number:      15,
	Title:       "Go is great",
	Description: "",
	CreatedAt:   time.Date(2018, 2, 21, 15, 51, 35, 0, time.UTC),
	User: &entity.User{
		Name: "Someone else",
	},
	VotesCount:    4,
	CommentsCount: 2,
	Status:        enum.PostOpen,
}

var duplicatePost = &entity.Post{
	Number:      20,
	Title:       "Go is easy",
	Description: "",
	CreatedAt:   time.Date(2018, 1, 12, 1, 46, 59, 0, time.UTC),
	User: &entity.User{
		Name: "Faceless",
	},
	VotesCount:    4,
	CommentsCount: 2,
	Status:        enum.PostDuplicate,
	Response: &entity.PostResponse{
		Text:        "This has already been suggested",
		RespondedAt: time.Date(2018, 3, 17, 10, 15, 42, 0, time.UTC),
		User: &entity.User{
			Name: "Arya Stark",
		},
		Original: &entity.OriginalPost{
			Number: 99,
			Title:  "Go is very easy",
		},
	},
	Tags: []string{"this-tag-has,comma"},
}
