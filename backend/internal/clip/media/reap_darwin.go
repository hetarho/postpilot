package media

import "time"

func enableChildReaper() error { return nil }

// Darwin reparents orphans to launchd, which reaps the killed descendants.
func reapChildren(_ int, _ time.Duration) {}
