package test

import (
	"time"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

var now = time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

func daysAgo(d int) time.Time { return now.AddDate(0, 0, -d) }

func ids(fs []zombie.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.ResourceID)
	}
	return out
}
