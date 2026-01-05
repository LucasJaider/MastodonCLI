package metrics

import (
	"fmt"
	"time"

	"mastodoncli/internal/mastodon"
)

type DailyMetric struct {
	Date    time.Time
	Label   string
	Follows int
	Likes   int
	Boosts  int
}

type Aggregator struct {
	windowStart time.Time
	windowEnd   time.Time
	byDay       map[string]*DailyMetric
}

func NewAggregator(days int, now time.Time) *Aggregator {
	if days < 1 {
		days = 1
	}
	end := truncateDay(now)
	start := end.AddDate(0, 0, -days+1)
	return &Aggregator{
		windowStart: start,
		windowEnd:   end,
		byDay:       make(map[string]*DailyMetric, days),
	}
}

func (a *Aggregator) WindowStart() time.Time {
	return a.windowStart
}

func (a *Aggregator) Add(notifications []mastodon.Notification) {
	for _, notification := range notifications {
		day := parseDay(notification.CreatedAt)
		if day.IsZero() {
			continue
		}
		if day.Before(a.windowStart) || day.After(a.windowEnd) {
			continue
		}
		key := day.Format("2006-01-02")
		metric, ok := a.byDay[key]
		if !ok {
			metric = &DailyMetric{Date: day, Label: day.Format("Jan 2")}
			a.byDay[key] = metric
		}
		switch notification.Type {
		case "follow":
			metric.Follows++
		case "favourite":
			metric.Likes++
		case "reblog":
			metric.Boosts++
		}
	}
}

func (a *Aggregator) AddGrouped(groups []mastodon.GroupedNotification) {
	for _, group := range groups {
		if group.LatestAt == "" {
			continue
		}
		day := parseDay(group.LatestAt)
		if day.IsZero() {
			continue
		}
		if day.Before(a.windowStart) || day.After(a.windowEnd) {
			continue
		}
		key := day.Format("2006-01-02")
		metric, ok := a.byDay[key]
		if !ok {
			metric = &DailyMetric{Date: day, Label: day.Format("Jan 2")}
			a.byDay[key] = metric
		}

		switch group.Type {
		case "follow":
			metric.Follows += group.Count
		case "favourite":
			metric.Likes += group.Count
		case "reblog":
			metric.Boosts += group.Count
		}
	}
}

func (a *Aggregator) Series() []DailyMetric {
	var series []DailyMetric
	for day := a.windowStart; !day.After(a.windowEnd); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if metric, ok := a.byDay[key]; ok {
			series = append(series, *metric)
		} else {
			series = append(series, DailyMetric{Date: day, Label: day.Format("Jan 2")})
		}
	}
	return series
}

func parseDay(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}
		}
	}
	return truncateDay(parsed.In(time.Local))
}

func truncateDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func FormatTotal(series []DailyMetric) string {
	var follows, likes, boosts int
	for _, day := range series {
		follows += day.Follows
		likes += day.Likes
		boosts += day.Boosts
	}
	return fmt.Sprintf("Follows %d · Likes %d · Boosts %d", follows, likes, boosts)
}

func FetchDailyMetrics(client *mastodon.Client, days int, progress func(scanned int)) ([]DailyMetric, error) {
	agg := NewAggregator(days, time.Now())
	const pageLimit = 40

	var maxID string
	scanned := 0
	for {
		page, err := client.GroupedNotificationsPage(pageLimit, maxID)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		agg.AddGrouped(page)
		scanned += len(page)
		if progress != nil {
			progress(scanned)
		}

		oldest := parseDay(page[len(page)-1].LatestAt)
		if !oldest.IsZero() && oldest.Before(agg.WindowStart()) {
			break
		}
		maxID = page[len(page)-1].MostRecent
	}

	return agg.Series(), nil
}
