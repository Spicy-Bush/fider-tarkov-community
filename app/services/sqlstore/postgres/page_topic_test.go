package postgres_test

import (
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestPageTopicsWithNullOptionalFields(t *testing.T) {
	f := newPostWorkflow(t)
	topic := &cmd.CreatePageTopic{Name: "Existing topic"}
	if err := bus.Dispatch(f.ctx, topic); err != nil {
		t.Fatal(err)
	}

	page := &cmd.CreatePage{
		Title:      "Existing Page",
		Content:    "Published content",
		Status:     entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
		Topics:     []int{topic.Result.ID},
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	if _, err := mediaFixtureSQL("UPDATE page_topics SET description=NULL, color=NULL WHERE id=$1", topic.Result.ID); err != nil {
		t.Fatal(err)
	}

	listed := &query.GetPageTopics{}
	loaded := &query.GetPageTopicByID{ID: topic.Result.ID}
	byID := &query.GetPageByID{ID: page.Result.ID}
	bySlug := &query.GetPageBySlug{Slug: page.Result.Slug}
	if err := bus.Dispatch(f.ctx, listed, loaded, byID, bySlug); err != nil {
		t.Fatal(err)
	}

	results := [][]*entity.PageTopic{
		listed.Result,
		{loaded.Result},
		byID.Result.Topics,
		bySlug.Result.Topics,
	}
	for _, topics := range results {
		if len(topics) != 1 {
			t.Fatalf("topics=%+v, want one topic", topics)
		}

		if topics[0].ID != topic.Result.ID || topics[0].Description != "" || topics[0].Color != "" {
			t.Fatalf("unexpected topic: %+v", topics[0])
		}
	}
}
