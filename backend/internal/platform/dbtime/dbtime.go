// Package dbtime owns how an instant is stored in a text column. SQLite has no date
// type, so every context writes the same fixed-width RFC3339 in UTC: the width matters
// because stored values are compared and ordered as strings.
package dbtime

import "time"

// Layout is RFC3339 with the fraction pinned to nine digits, so a value with zero
// nanoseconds is as wide as one with many and text order equals time order.
const Layout = "2006-01-02T15:04:05.000000000Z07:00"

// Format normalizes to UTC before formatting so stored values sort against each other
// regardless of the offset the caller's clock carried.
func Format(t time.Time) string { return t.UTC().Format(Layout) }

// Parse reads with RFC3339Nano rather than Layout: it accepts any RFC3339 fraction
// width, so a row written before the width was pinned still loads.
func Parse(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
