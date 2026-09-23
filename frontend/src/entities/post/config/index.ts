/** The post context's own generation ceilings (ARCH-21), moved out of
 *  `shared/config` with T258. The server owns the same values and stays
 *  authoritative; these are what lets a field refuse before the round trip. */

export const POST_TARGET_LENGTH_MIN = 100
export const POST_TARGET_LENGTH_MAX = 10_000

/** What 목표 글자 수 사용 fills the empty field with. A ticked checkbox over a blank number input is
 *  an invalid form the user did not ask for — the field renders its range error before anyone has
 *  typed a character — so the box arrives with a usable value already in it, roughly the length of
 *  an ordinary blog post. It is a STARTING POINT, not a floor: the field stays free between
 *  POST_TARGET_LENGTH_MIN and POST_TARGET_LENGTH_MAX, and a value already typed is never
 *  overwritten by it. */
export const POST_TARGET_LENGTH_DEFAULT = 1_000

/** How many tags a run asks for (POST-63). Unlike the length there is no "natural" count to
 *  opt into: a post never saved with one reads as the default, and the field is always shown.
 *  The server owns the same three values (`post.TagCountRange`) and refuses anything outside
 *  the range with POST_TAG_COUNT_INVALID. */
export const POST_TAG_COUNT_DEFAULT = 4
export const POST_TAG_COUNT_MIN = 1
export const POST_TAG_COUNT_MAX = 10

/** The longest Naver Blog address a post stores, in code points. Mirrors the server's
 *  `post.PublishedURLMaxChars` (ARCH-21), which stays authoritative. */
export const POST_PUBLISHED_URL_MAX_CHARS = 2048
