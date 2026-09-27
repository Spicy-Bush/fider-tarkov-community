package enum_test

import (
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestPostStatusRejectsUnknownNames(t *testing.T) {
	for _, value := range []string{"", "unknown", "Open", "0", " completed "} {
		t.Run(value, func(t *testing.T) {
			status := enum.PostStarted
			if err := status.UnmarshalText([]byte(value)); err == nil {
				t.Fatalf("accepted invalid status %q", value)
			}
			if status != enum.PostStarted {
				t.Fatalf("invalid input changed existing status to %v", status)
			}
		})
	}
}

func TestPostStatusNamesRoundTrip(t *testing.T) {
	for _, status := range []enum.PostStatus{
		enum.PostOpen,
		enum.PostStarted,
		enum.PostCompleted,
		enum.PostDeclined,
		enum.PostPlanned,
		enum.PostDuplicate,
		enum.PostDeleted,
		enum.PostArchived,
	} {
		text, err := status.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var decoded enum.PostStatus
		if err := decoded.UnmarshalText(text); err != nil {
			t.Fatal(err)
		}
		if decoded != status {
			t.Fatalf("status %v decoded as %v", status, decoded)
		}
	}
}
