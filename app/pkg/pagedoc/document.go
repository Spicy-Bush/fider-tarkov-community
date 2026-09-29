package pagedoc

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/reearth/ygo/crdt"
)

const MaxStateBytes = 4 << 20

type Result struct {
	State       []byte
	Update      []byte
	StateVector []byte
	Page        *cmd.UpdatePage
}

func New(page *entity.Page) []byte {
	document := crdt.New()
	document.Transact(func(transaction *crdt.Transaction) {
		for name, value := range map[string]string{
			"title":           page.Title,
			"slug":            page.Slug,
			"content":         page.Content,
			"excerpt":         page.Excerpt,
			"metaDescription": page.MetaDescription,
		} {
			transaction.GetText(name).Insert(transaction, 0, value, nil)
		}

		var parent any
		if page.ParentPageID != nil {
			parent = *page.ParentPageID
		}

		scheduledFor := ""
		if page.ScheduledFor != nil {
			scheduledFor = page.ScheduledFor.Format(time.RFC3339Nano)
		}

		settings := transaction.GetMap("settings")
		for name, value := range map[string]any{
			"status":             string(page.Status),
			"visibility":         string(page.Visibility),
			"parentPageId":       parent,
			"bannerImageBKey":    page.BannerImageBKey,
			"allowComments":      page.AllowComments,
			"allowCommentImages": page.AllowCommentImages,
			"allowReactions":     page.AllowReactions,
			"showToc":            page.ShowTOC,
			"scheduledFor":       scheduledFor,
		} {
			settings.Set(transaction, name, value)
		}

		for _, topic := range page.Topics {
			transaction.GetMap("topics").Set(transaction, strconv.Itoa(topic.ID), true)
		}
		for _, tag := range page.Tags {
			transaction.GetMap("tags").Set(transaction, strconv.Itoa(tag.ID), true)
		}
		for position, author := range page.Authors {
			transaction.GetMap("authors").Set(transaction, strconv.Itoa(author.ID), position)
		}
		for _, role := range page.AllowedRoles {
			transaction.GetMap("allowedRoles").Set(transaction, role, true)
		}
	})

	return crdt.EncodeStateAsUpdateV1(document, nil)
}

func load(state []byte) (*crdt.Doc, error) {
	document := crdt.New(crdt.WithMaxPendingItems(4096))
	if err := crdt.ApplyUpdateV1(document, state, nil); err != nil {
		return nil, fmt.Errorf("read collaborative Page: %w", err)
	}
	return document, nil
}

func Read(state []byte) (*cmd.UpdatePage, error) {
	document, err := load(state)
	if err != nil {
		return nil, err
	}
	return materialize(document)
}

func StateVector(state []byte) ([]byte, error) {
	document, err := load(state)
	if err != nil {
		return nil, err
	}
	return crdt.EncodeStateVectorV1(document), nil
}

func ClearBanner(state []byte) ([]byte, error) {
	document, err := load(state)
	if err != nil {
		return nil, err
	}

	document.Transact(func(transaction *crdt.Transaction) {
		transaction.GetMap("settings").Set(transaction, "bannerImageBKey", "")
	})
	return crdt.EncodeStateAsUpdateV1(document, nil), nil
}

func SetStatus(state []byte, status entity.PageStatus) ([]byte, error) {
	document, err := load(state)
	if err != nil {
		return nil, err
	}
	current, _ := document.GetMap("settings").Get("status")
	if current == string(status) {
		return state, nil
	}

	document.Transact(func(transaction *crdt.Transaction) {
		transaction.GetMap("settings").Set(transaction, "status", string(status))
	})
	return crdt.EncodeStateAsUpdateV1(document, nil), nil
}

func Merge(state, update, stateVector []byte) (*Result, error) {
	if len(update) > MaxStateBytes || len(stateVector) > 64<<10 {
		return nil, validate.Failed("The Page update is too large.")
	}

	var vector crdt.StateVector
	if len(stateVector) != 0 {
		var err error
		vector, err = crdt.DecodeStateVectorV1(stateVector)
		if err != nil {
			return nil, validate.Failed("Invalid Page state vector.")
		}
	}

	document, err := load(state)
	if err != nil {
		return nil, err
	}
	if err := crdt.ApplyUpdateV1(document, update, nil); err != nil {
		return nil, validate.Failed("Invalid Page update.")
	}

	// The sender includes every edit newer than the last acknowledged server vector.
	pending := document.PendingStats()
	if pending.Items != 0 || pending.DeleteRanges != 0 {
		return nil, validate.Failed("The Page update is missing earlier edits. Reconnect to synchronise them.")
	}
	for client, clock := range document.StateVector() {
		if client > 1<<53-1 || clock > 1<<53-1 {
			return nil, validate.Failed("The Page update contains an invalid editing identity.")
		}
	}

	page, err := materialize(document)
	if err != nil {
		return nil, err
	}

	stored := crdt.EncodeStateAsUpdateV1(document, nil)
	if len(stored) > MaxStateBytes {
		return nil, validate.Failed("The Page's editing history is too large.")
	}

	return &Result{
		State:       stored,
		Update:      crdt.EncodeStateAsUpdateV1(document, vector),
		StateVector: crdt.EncodeStateVectorV1(document),
		Page:        page,
	}, nil
}

func materialize(document *crdt.Doc) (*cmd.UpdatePage, error) {
	page := &cmd.UpdatePage{}
	for _, field := range []struct {
		name  string
		value *string
		limit int
	}{
		{"title", &page.Title, 200},
		{"slug", &page.Slug, 200},
		{"content", &page.Content, 50000},
		{"excerpt", &page.Excerpt, 500},
		{"metaDescription", &page.MetaDescription, 300},
	} {
		parts := document.GetText(field.name).ToDelta()
		if len(parts) > 1 {
			return nil, validate.Failed("Page text must contain plain Markdown.")
		}

		if len(parts) == 1 {
			value, ok := parts[0].Insert.(string)
			if !ok || len(parts[0].Attributes) != 0 {
				return nil, validate.Failed("Page text must contain plain Markdown.")
			}
			*field.value = value
		}

		if len(*field.value) > field.limit {
			result := validate.Success()
			result.AddFieldFailure(field.name, fmt.Sprintf("Use at most %d bytes.", field.limit))
			return nil, result
		}
	}

	settings := document.GetMap("settings").Entries()
	for name, target := range map[string]*bool{
		"allowComments":      &page.AllowComments,
		"allowCommentImages": &page.AllowCommentImages,
		"allowReactions":     &page.AllowReactions,
		"showToc":            &page.ShowTOC,
	} {
		value, ok := settings[name].(bool)
		if !ok {
			return nil, validate.Failed("Invalid Page setting: " + name)
		}
		*target = value
		delete(settings, name)
	}

	status, ok := settings["status"].(string)
	if !ok || (status != "draft" && status != "published" && status != "unpublished" && status != "scheduled") {
		return nil, validate.Failed("Invalid Page status.")
	}
	page.Status = entity.PageStatus(status)
	delete(settings, "status")

	visibility, ok := settings["visibility"].(string)
	if !ok || (visibility != "public" && visibility != "private" && visibility != "unlisted") {
		return nil, validate.Failed("Invalid Page visibility.")
	}
	page.Visibility = entity.PageVisibility(visibility)
	delete(settings, "visibility")

	banner, ok := settings["bannerImageBKey"].(string)
	if !ok || len(banner) > 255 || (banner != "" && blob.ValidateKey(banner) != nil) {
		return nil, validate.Failed("Invalid Page banner.")
	}
	page.BannerImage = &dto.ImageUpload{BlobKey: banner, Remove: banner == ""}
	delete(settings, "bannerImageBKey")

	parent, exists := settings["parentPageId"]
	if !exists {
		return nil, validate.Failed("Invalid parent Page.")
	}
	if parent != nil {
		number, ok := parent.(int64)
		if !ok || number < 1 || number > math.MaxInt32 {
			return nil, validate.Failed("Invalid parent Page.")
		}
		id := int(number)
		page.ParentPageID = &id
	}
	delete(settings, "parentPageId")

	scheduledFor, ok := settings["scheduledFor"].(string)
	if !ok {
		return nil, validate.Failed("Invalid Page schedule.")
	}
	if scheduledFor != "" {
		scheduled, err := time.Parse(time.RFC3339, scheduledFor)
		if err != nil {
			return nil, validate.Failed("Invalid Page schedule.")
		}
		page.ScheduledFor = &scheduled
	}
	delete(settings, "scheduledFor")
	if len(settings) > 0 {
		return nil, validate.Failed("Unknown Page settings.")
	}

	for name, target := range map[string]*[]int{
		"topics": &page.Topics,
		"tags":   &page.Tags,
	} {
		entries := document.GetMap(name).Entries()
		if len(entries) > 1000 {
			return nil, validate.Failed("Too many selected " + name + ".")
		}
		for key, value := range entries {
			id, err := strconv.Atoi(key)
			if err != nil || id < 1 || id > math.MaxInt32 || strconv.Itoa(id) != key || value != true {
				return nil, validate.Failed("Invalid selected " + name + ".")
			}
			*target = append(*target, id)
		}
		sort.Ints(*target)
	}

	authors := document.GetMap("authors").Entries()
	if len(authors) > 1000 {
		return nil, validate.Failed("Too many selected authors.")
	}

	type authorPosition struct {
		id       int
		position int64
	}
	ordered := make([]authorPosition, 0, len(authors))
	for key, value := range authors {
		id, err := strconv.Atoi(key)
		position, validPosition := value.(int64)
		if err != nil || id < 1 || id > math.MaxInt32 || strconv.Itoa(id) != key ||
			!validPosition || position < 0 || position > math.MaxInt32 {
			return nil, validate.Failed("Invalid selected author.")
		}
		ordered = append(ordered, authorPosition{id: id, position: position})
	}

	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].position == ordered[right].position {
			return ordered[left].id < ordered[right].id
		}
		return ordered[left].position < ordered[right].position
	})
	for _, author := range ordered {
		page.Authors = append(page.Authors, author.id)
	}

	for name, value := range document.GetMap("allowedRoles").Entries() {
		var role enum.Role
		_ = role.UnmarshalText([]byte(name))
		if role.String() == "" || value != true {
			return nil, validate.Failed("Invalid Page role.")
		}
		page.AllowedRoles = append(page.AllowedRoles, name)
	}
	sort.Strings(page.AllowedRoles)

	return page, nil
}
