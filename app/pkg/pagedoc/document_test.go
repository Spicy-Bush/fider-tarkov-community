package pagedoc_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pagedoc"
	"github.com/reearth/ygo/crdt"
	"github.com/stretchr/testify/require"
)

func workingPage() *entity.Page {
	parent := 12
	scheduled := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entity.Page{
		Title:              "Shared Page",
		Slug:               "shared-page",
		Content:            "Hello 🌍",
		Excerpt:            "A shared introduction",
		MetaDescription:    "Description",
		CanonicalURL:       "/pages/shared-page",
		Status:             entity.PageStatusScheduled,
		Visibility:         entity.PageVisibilityPrivate,
		AllowedRoles:       []string{"administrator", "collaborator"},
		ParentPageID:       &parent,
		AllowComments:      true,
		AllowCommentImages: true,
		AllowReactions:     true,
		ShowTOC:            true,
		ScheduledFor:       &scheduled,
		BannerImageBKey:    "pages/banner.png",
		Authors:            []*entity.User{{ID: 9}, {ID: 4}},
		Topics:             []*entity.PageTopic{{ID: 2}},
		Tags:               []*entity.PageTag{{ID: 3}},
	}
}

func replica(t testing.TB, state []byte, id crdt.ClientID) *crdt.Doc {
	t.Helper()
	document := crdt.New(crdt.WithClientID(id))
	require.NoError(t, crdt.ApplyUpdateV1(document, state, nil))
	return document
}

func TestPageDocumentPreservesFields(t *testing.T) {
	page := workingPage()
	stored := pagedoc.New(page)
	read, err := pagedoc.Read(stored)
	require.NoError(t, err)

	require.Equal(t, page.Title, read.Title)
	require.Equal(t, page.Slug, read.Slug)
	require.Equal(t, page.Content, read.Content)
	require.Equal(t, page.Excerpt, read.Excerpt)
	require.Equal(t, page.MetaDescription, read.MetaDescription)
	require.Equal(t, page.Status, read.Status)
	require.Equal(t, page.Visibility, read.Visibility)
	require.Equal(t, page.AllowedRoles, read.AllowedRoles)
	require.Equal(t, page.ParentPageID, read.ParentPageID)
	require.Equal(t, page.ScheduledFor, read.ScheduledFor)
	require.True(t, read.AllowComments && read.AllowCommentImages && read.AllowReactions && read.ShowTOC)
	require.Equal(t, page.BannerImageBKey, read.BannerImage.BlobKey)
	require.Equal(t, []int{9, 4}, read.Authors)
	require.Equal(t, []int{2}, read.Topics)
	require.Equal(t, []int{3}, read.Tags)
}

func TestConcurrentPageEditsAndDuplicateDelivery(t *testing.T) {
	initial := pagedoc.New(workingPage())
	alice := replica(t, initial, 100)
	bob := replica(t, initial, 200)
	baseline := alice.StateVector()

	alice.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Alice: ", nil)
		transaction.GetMap("topics").Set(transaction, "5", true)
		transaction.GetMap("authors").Set(transaction, "6", 2)
	})
	bob.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 8, " from Bob", nil)
		transaction.GetMap("tags").Set(transaction, "7", true)
		transaction.GetMap("authors").Set(transaction, "7", 2)
	})

	updates := [][]byte{
		crdt.EncodeStateAsUpdateV1(alice, baseline),
		crdt.EncodeStateAsUpdateV1(bob, baseline),
	}
	for _, order := range [][]int{{0, 1, 0}, {1, 0, 1}} {
		state := initial
		for _, index := range order {
			result, err := pagedoc.Merge(state, updates[index], nil)
			require.NoError(t, err)
			state = result.State
		}

		read, err := pagedoc.Read(state)
		require.NoError(t, err)
		require.Equal(t, "Alice: Hello 🌍 from Bob", read.Content)
		require.Equal(t, []int{2, 5}, read.Topics)
		require.Equal(t, []int{3, 7}, read.Tags)
		require.Equal(t, []int{9, 4, 6, 7}, read.Authors)
	}
}

func TestPageBannerDeletionMergesWithPendingText(t *testing.T) {
	initial := pagedoc.New(workingPage())
	client := replica(t, initial, 100)
	baseline := client.StateVector()
	client.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("title").Insert(transaction, 0, "Edited ", nil)
	})

	deleted, err := pagedoc.ClearBanner(initial)
	require.NoError(t, err)
	result, err := pagedoc.Merge(deleted, crdt.EncodeStateAsUpdateV1(client, baseline), crdt.EncodeStateVectorV1(client))
	require.NoError(t, err)
	require.Equal(t, "Edited Shared Page", result.Page.Title)
	require.True(t, result.Page.BannerImage.Remove)

	require.NoError(t, crdt.ApplyUpdateV1(client, result.Update, nil))
	banner, exists := client.GetMap("settings").Get("bannerImageBKey")
	require.True(t, exists)
	require.Equal(t, "", banner)
}

func TestPageDocumentRejectsInvalidChangesWithoutLosingRecovery(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*crdt.Transaction)
	}{
		{"too much text", func(transaction *crdt.Transaction) {
			transaction.GetText("content").Insert(transaction, 0, strings.Repeat("x", 50001), nil)
		}},
		{"embedded object", func(transaction *crdt.Transaction) {
			transaction.GetText("content").InsertEmbed(transaction, 0, map[string]any{"image": "not Markdown"}, nil)
		}},
		{"formatted text", func(transaction *crdt.Transaction) {
			transaction.GetText("content").Insert(transaction, 0, "bold", crdt.Attributes{"bold": true})
		}},
		{"invalid setting", func(transaction *crdt.Transaction) {
			transaction.GetMap("settings").Set(transaction, "allowComments", "true")
		}},
		{"missing setting", func(transaction *crdt.Transaction) {
			transaction.GetMap("settings").Delete(transaction, "status")
		}},
		{"invalid role", func(transaction *crdt.Transaction) {
			transaction.GetMap("allowedRoles").Set(transaction, "owner", true)
		}},
		{"invalid relation", func(transaction *crdt.Transaction) {
			transaction.GetMap("topics").Set(transaction, "-1", true)
		}},
		{"invalid author position", func(transaction *crdt.Transaction) {
			transaction.GetMap("authors").Set(transaction, "9", -1)
		}},
		{"noninteger author position", func(transaction *crdt.Transaction) {
			transaction.GetMap("authors").Set(transaction, "9", 0.5)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			initial := pagedoc.New(workingPage())
			unchanged := bytes.Clone(initial)
			client := replica(t, initial, 100)
			baseline := client.StateVector()
			client.Transact(test.change)

			_, err := pagedoc.Merge(initial, crdt.EncodeStateAsUpdateV1(client, baseline), nil)
			require.Error(t, err)
			require.Equal(t, unchanged, initial)

			healthy := replica(t, initial, 200)
			healthy.Transact(func(transaction *crdt.Transaction) {
				transaction.GetText("content").Insert(transaction, 0, "Continued: ", nil)
			})
			result, err := pagedoc.Merge(initial, crdt.EncodeStateAsUpdateV1(healthy, baseline), nil)
			require.NoError(t, err)
			require.Equal(t, "Continued: Hello 🌍", result.Page.Content)
		})
	}
}

func TestPageDocumentRejectsIncompleteUpdates(t *testing.T) {
	initial := pagedoc.New(workingPage())
	client := replica(t, initial, 100)
	baseline := client.StateVector()
	client.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "First ", nil)
	})
	partial := client.StateVector()
	client.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Second ", nil)
	})

	_, err := pagedoc.Merge(initial, crdt.EncodeStateAsUpdateV1(client, partial), nil)
	require.Error(t, err)

	result, err := pagedoc.Merge(initial, crdt.EncodeStateAsUpdateV1(client, baseline), nil)
	require.NoError(t, err)
	require.Equal(t, "Second First Hello 🌍", result.Page.Content)
}

func TestPageDocumentEditingIdentitiesFitBrowserNumbers(t *testing.T) {
	initial := pagedoc.New(workingPage())
	for _, id := range []crdt.ClientID{1<<53 - 1, 1 << 53} {
		client := replica(t, initial, id)
		baseline := client.StateVector()
		client.Transact(func(transaction *crdt.Transaction) {
			transaction.GetText("content").Insert(transaction, 0, "Edited ", nil)
		})

		_, err := pagedoc.Merge(initial, crdt.EncodeStateAsUpdateV1(client, baseline), nil)
		if id == 1<<53-1 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func FuzzPageDocumentUpdate(f *testing.F) {
	state := pagedoc.New(workingPage())
	f.Add([]byte{0, 0})
	f.Add(state)
	f.Add([]byte{255, 255, 255, 255, 255, 255, 255, 255, 127})
	f.Fuzz(func(t *testing.T, update []byte) {
		if len(update) > 4096 {
			t.Skip()
		}
		_, _ = pagedoc.Merge(state, update, nil)
	})
}

func BenchmarkPageDocumentSync(b *testing.B) {
	page := workingPage()
	page.Content = strings.Repeat("A paragraph of Markdown.\n", 1900)
	state := pagedoc.New(page)
	client := replica(b, state, 100)
	baseline := client.StateVector()
	client.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 24000, "🌍", nil)
	})
	update := crdt.EncodeStateAsUpdateV1(client, baseline)
	vector := crdt.EncodeStateVectorV1(client)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := pagedoc.Merge(state, update, vector); err != nil {
			b.Fatal(err)
		}
	}
}
