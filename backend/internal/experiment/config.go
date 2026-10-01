package experiment

import "time"

const (
	LeaderboardInitialRating  = 1500
	LeaderboardKFactor        = 32
	LeaderboardMinEvaluations = 3
	// The fixed opponent a dismissed candidate loses to: it never moves and is never ranked
	// (MODEL-38).
	LeaderboardDismissalReference = 1500
)

// The leaderboard windows are rolling, measured back from the moment of the request rather
// than aligned to a calendar: a calendar day would empty the board every midnight, and a
// board nobody can read the morning after a comparison is not a board (MODEL-38).
//
// Code-owned rather than env-configured: the three lengths are the product's vocabulary
// (일간 · 주간 · 월간), and an operator changing one would silently rewrite what every
// rating means.
const (
	LeaderboardWindowDay   = 24 * time.Hour
	LeaderboardWindowWeek  = 7 * 24 * time.Hour
	LeaderboardWindowMonth = 30 * 24 * time.Hour
)
