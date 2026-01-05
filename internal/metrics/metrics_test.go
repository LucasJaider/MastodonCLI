package metrics

import (
	"testing"
	"time"

	"mastodoncli/internal/mastodon"
)

func TestAggregatorSeries(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	agg := NewAggregator(3, now)

	groups := []mastodon.GroupedNotification{
		{Type: "follow", LatestAt: "2025-01-10T08:00:00Z", Count: 1},
		{Type: "favourite", LatestAt: "2025-01-10T09:00:00Z", Count: 2},
		{Type: "reblog", LatestAt: "2025-01-09T12:00:00Z", Count: 3},
		{Type: "mention", LatestAt: "2025-01-08T08:00:00Z", Count: 5},
		{Type: "follow", LatestAt: "2024-12-31T12:00:00Z", Count: 7},
	}

	agg.AddGrouped(groups)
	series := agg.Series()

	if len(series) != 3 {
		t.Fatalf("expected 3 days, got %d", len(series))
	}

	if series[2].Follows != 1 || series[2].Likes != 2 || series[2].Boosts != 0 {
		t.Fatalf("unexpected totals for day 3: %+v", series[2])
	}
	if series[1].Boosts != 3 {
		t.Fatalf("unexpected boosts for day 2: %+v", series[1])
	}
	if series[0].Follows != 0 || series[0].Likes != 0 || series[0].Boosts != 0 {
		t.Fatalf("expected zeros for day 1: %+v", series[0])
	}
}
