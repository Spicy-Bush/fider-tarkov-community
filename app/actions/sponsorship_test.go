package actions

import "testing"

func TestNormalizeCreativeImageURLs(t *testing.T) {
	got := NormalizeCreativeImageURLs(map[string]string{
		"feed_native":  " https://a.test/f.jpg ",
		"nope":         "https://a.test/x.jpg",
		"sidebar_top":  "",
		"pages_header": "https://a.test/p.jpg",
	})
	if len(got) != 2 || got["feed_native"] != "https://a.test/f.jpg" || got["pages_header"] != "https://a.test/p.jpg" {
		t.Fatalf("unexpected: %#v", got)
	}
	if got := NormalizeCreativeImageURLs(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil input should yield empty map, got %#v", got)
	}
}
