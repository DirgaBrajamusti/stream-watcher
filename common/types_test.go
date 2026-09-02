package common

import (
	"testing"
)

func TestJobStore(t *testing.T) {
	store := NewJobStore()
	live := ChannelLive{ChannelID: "c1", VideoID: "v1", Title: "Test"}
	job := NewDownloadJob("v1", live, StatusWaiting, "/tmp")

	if _, ok := store.Add(job); !ok {
		t.Fatal("expected add to succeed")
	}

	if _, ok := store.Get("v1"); !ok {
		t.Fatal("expected job to exist")
	}

	if !store.IsChannelBusy("c1") {
		t.Fatal("expected channel to be busy")
	}

	store.SetStatus("v1", StatusFinished, "")

	if store.IsChannelBusy("c1") {
		t.Fatal("expected channel not to be busy after finish")
	}

	if !store.Exists("v1") {
		t.Fatal("finished jobs should still exist in the store")
	}

	store.SetStatus("v1", StatusError, "boom")
	if store.Exists("v1") {
		t.Fatal("errored jobs should not count as existing")
	}
}

func TestMatchFilters(t *testing.T) {
	filters := []string{`(?i)live`, `test`}
	if !MatchFilters("This is a live stream", filters) {
		t.Fatal("expected title to match filter")
	}
	if MatchFilters("Nothing relevant", filters) {
		t.Fatal("expected title not to match")
	}
}

func TestMatchFiltersEmpty(t *testing.T) {
	if !MatchFilters("anything", []string{""}) {
		t.Fatal("empty filter should match everything")
	}
}

func TestMatchFiltersEmptyList(t *testing.T) {
	if !MatchFilters("anything", []string{}) {
		t.Fatal("empty filter list should match everything")
	}
	if !MatchFilters("anything", nil) {
		t.Fatal("nil filter list should match everything")
	}
}
