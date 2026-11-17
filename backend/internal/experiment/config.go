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

// A photo observation test's photos (MODEL-97) reuse the post upload limits by name: the PUT
// and GET lifetimes, the object size ceiling and the per-owner photo count come from the same
// configuration as a post photo (→POST-31, →POST-36). What this context owns is the prefix, the
// wire type and how long an upload no test has claimed may wait.
const (
	// TestPhotoPrefix is the object key prefix only the test photo sweep lists; the post sweep
	// lists `posts/` and the voice sweep `voices/`.
	TestPhotoPrefix = "tests/"
	// TestPhotoContentType is what the presigned PUT is signed for: the browser re-encodes every
	// photo to JPEG before it uploads (→POST-33).
	TestPhotoContentType = "image/jpeg"
	// DefaultTestPhotoTTL is how long an unattached test photo lives when WRITING_TEST_PHOTO_TTL
	// does not say otherwise.
	DefaultTestPhotoTTL = 24 * time.Hour
	// maxTestPhotoDimension bounds the width and height a client may report, the same ceiling
	// the post context enforces (→POST-36).
	maxTestPhotoDimension = 20000
	// testPhotoSweepBatch bounds one list of expired rows; a larger backlog takes more rounds.
	testPhotoSweepBatch = 200
)
