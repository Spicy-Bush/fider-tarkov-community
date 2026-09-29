package entity

import (
	"slices"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type FeedbackExportMode string

const (
	FeedbackExportTopVoted      FeedbackExportMode = "top"
	FeedbackExportControversial FeedbackExportMode = "controversial"
	FeedbackExportDiscussed     FeedbackExportMode = "discussed"
	FeedbackExportRandom        FeedbackExportMode = "random"
)

func (mode FeedbackExportMode) IsValid() bool {
	switch mode {
	case FeedbackExportTopVoted, FeedbackExportControversial, FeedbackExportDiscussed, FeedbackExportRandom:
		return true
	}
	return false
}

const (
	MaxFeedbackExportSections = 10
	MaxFeedbackExportPicks    = 5
	MaxFeedbackExportPickSize = 200
	MaxFeedbackExportRows     = 500
)

var FeedbackExportStatuses = []enum.PostStatus{
	enum.PostOpen, enum.PostStarted, enum.PostCompleted, enum.PostDeclined, enum.PostPlanned, enum.PostDuplicate,
}

type FeedbackExportPick struct {
	Mode        FeedbackExportMode `json:"mode"`
	Count       int                `json:"count"`
	MinComments int                `json:"minComments"`
}

type FeedbackExportSection struct {
	Name        string               `json:"name"`
	IncludeTags []int                `json:"includeTags"`
	ExcludeTags []int                `json:"excludeTags"`
	Statuses    []enum.PostStatus    `json:"statuses"`
	MaxAgeDays  int                  `json:"maxAgeDays"`
	MinVotes    *int                 `json:"minVotes"`
	Picks       []FeedbackExportPick `json:"picks"`
}

type FeedbackExportRecipe struct {
	Sections []FeedbackExportSection `json:"sections"`
}

func (recipe *FeedbackExportRecipe) CanonicalizeFilters() {
	for i := range recipe.Sections {
		section := &recipe.Sections[i]
		slices.Sort(section.Statuses)
		section.Statuses = slices.Compact(section.Statuses)

		slices.Sort(section.IncludeTags)
		section.IncludeTags = slices.Compact(section.IncludeTags)
		if section.IncludeTags == nil {
			section.IncludeTags = []int{}
		}

		slices.Sort(section.ExcludeTags)
		section.ExcludeTags = slices.Compact(section.ExcludeTags)
		if section.ExcludeTags == nil {
			section.ExcludeTags = []int{}
		}
	}
}

func (recipe FeedbackExportRecipe) Equal(other FeedbackExportRecipe) bool {
	if len(recipe.Sections) != len(other.Sections) {
		return false
	}

	for i, section := range recipe.Sections {
		otherSection := other.Sections[i]
		if section.Name != otherSection.Name || !section.SameFilters(otherSection) || !slices.Equal(section.Picks, otherSection.Picks) {
			return false
		}
	}

	return true
}

func (section FeedbackExportSection) SameFilters(other FeedbackExportSection) bool {
	if section.MaxAgeDays != other.MaxAgeDays || (section.MinVotes == nil) != (other.MinVotes == nil) {
		return false
	}

	if section.MinVotes != nil && *section.MinVotes != *other.MinVotes {
		return false
	}

	return slices.Equal(section.Statuses, other.Statuses) &&
		slices.Equal(section.IncludeTags, other.IncludeTags) &&
		slices.Equal(section.ExcludeTags, other.ExcludeTags)
}

type FeedbackExportPresetContent struct {
	Name   string               `json:"name"`
	Recipe FeedbackExportRecipe `json:"recipe"`
}

type FeedbackExportPresetUpdate struct {
	Preset    *FeedbackExportPreset `json:"preset"`
	Conflicts []string              `json:"conflicts"`
}

type FeedbackExportPreset struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Recipe    FeedbackExportRecipe `json:"recipe"`
	UpdatedAt time.Time            `json:"updatedAt"`
	UpdatedBy string               `json:"updatedBy"`
}

type FeedbackExportRow struct {
	Number   int                `json:"number"`
	Title    string             `json:"title"`
	Slug     string             `json:"slug"`
	Votes    int                `json:"votes"`
	Comments int                `json:"comments"`
	TagIDs   []int              `json:"tagIds"`
	Pick     FeedbackExportMode `json:"pick"`
}
