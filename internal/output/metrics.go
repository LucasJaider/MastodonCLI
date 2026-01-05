package output

import (
	"fmt"

	"mastodoncli/internal/metrics"
)

func PrintDailyMetrics(series []metrics.DailyMetric) {
	if len(series) == 0 {
		fmt.Println("No metrics returned.")
		return
	}

	for _, day := range series {
		fmt.Printf("%-6s  F:%d  L:%d  B:%d\n", day.Label, day.Follows, day.Likes, day.Boosts)
	}
	fmt.Println(metrics.FormatTotal(series))
}
