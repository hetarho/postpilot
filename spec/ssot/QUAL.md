# QUAL published-post measurement and 분야 phrases
> r3 | Measure what code can count about a post while it can still be changed and across the account's published Naver posts, offer one rule per finding, and collect each 분야's top-ranking phrases from the official search API.

## decisions
- QUAL-1 [o] QUAL owns the two observations the product makes outside a single post: how an account's published posts look next to each other, and which phrases rank for a 분야 on Naver
- QUAL-2 [o] measurement has two layers: a post's own numbers are computed for any post that has content, while the account aggregate counts 발행됨 posts only ← a number that changes what a later post is told must come from posts that actually went up, but the author needs this post's own numbers while there is still time to act on them
- QUAL-3 [o] a post's own measurement is computed for its current content revision and recomputed when that revision changes; the aggregate is recomputed when a URL is pasted or cleared (→POST-73 →POST-75)
- QUAL-4 [o] a post's self-only numbers (M3, M4) are stored against the revision they describe, computed on first read of that revision; M2, which also depends on the published window, is computed at read against the current window; the aggregate is derived at read from 발행됨 posts ← nothing then goes stale when another post is published, and no post write has to call into QUAL
- QUAL-5 [o] four metrics ship, each a pass/warn badge with its own minimum published count, never a weighted score ← no published weighting exists for any of them
- QUAL-6 [o] every band is the product's own and every screen carrying one says so ← the only quantitative rule Naver publishes is that a keyword repeated twice or more in a title risks a penalty
- QUAL-7 [o] M1 제목 도배율 is an account metric with no per-post value: over the content titles of the account's last 100 발행됨 posts, the share containing the account's most frequent noun, where the candidate nouns are those the write pass returned for those posts (→GEN-55), a Korean title contains a noun when one of its 어절 starts with it, an English one when a word equals it case-insensitively, and the most frequent noun is the one contained in the most titles; minimum 10 published posts ← a saturation over one title is not a quantity, and matching against the text keeps a hand-edited title measurable
- QUAL-8 [o] M2 글 간 고정 문구: per post, the share of its characters standing inside a run of 8 or more consecutive 어절 that also appears verbatim in one of the account's last 20 발행됨 posts; the account value is the median over those posts; minimum 3
- QUAL-9 [o] M3 글 안 반복과 제목 관련성: per post, the share of body noun occurrences taken by its most frequent noun, counting the post's returned nouns by the containment rule of QUAL-7, and the share of those nouns still contained in the title that the body also contains; a post with no returned nouns has no M3 (→QUAL-40); the account value is each median over the last 20; minimum 1
- QUAL-10 [o] M4 분량·구성: per post the character count, photo count, distinct `Block` types used and average sentence length (in characters for Korean, in words for English), the photo count being IMAGE blocks that carry a file; only the count of distinct block types carries a band; minimum 3 published posts ← Naver publishes no target length, so the other three are context shown beside the badge rather than something to pass or fail
- QUAL-11 [o] the bands: M1 warns above 30%, M2 above a 10% median, M3 above an 8% repetition share or below 50% title relevance, and M4 at a median of 2 or fewer distinct block types
- QUAL-12 [o] a metric below its minimum renders a line naming that minimum, not an absent control; the four row states are over band, within band, below minimum and absent (→POST-81) ← a control that is simply missing cannot be told apart from one that broke
- QUAL-13 [o] each metric owns one rule text, a code-owned constant a 지침 may override
- QUAL-14 [o] M1's and M2's rule texts name the measured phrase itself rather than instructing against overlap in the abstract; at the account level M2 names the run standing in the most of the last 20 발행됨 posts, ties broken by the longer run and then the earliest ← a model acts on a named string reliably and on "겹치는 표현" barely at all
- QUAL-15 [o] a phrase is named by M1 only while it stands in the account's own recent titles, so an account with nothing published bans nothing
- QUAL-16 [o] a measurement calls no provider and costs no credit; the nouns it reads ride the write call the owner already pays for (→GEN-55)
- QUAL-17 [o] the 분야 phrase batch runs about once a day per 분야 against the official 네이버 검색 API (`/v1/search/blog.json`, `sort=sim`), taking 300 results per field as three pages of 100, product-side and never as a per-owner live call ← no per-owner key is needed and the 25,000/day quota stays inside the product's control
- QUAL-18 [o] the corpus is the returned `title` and `description` only ← Naver's robots.txt and terms forbid crawling blog.naver.com and name the Open API as the sanctioned route
- QUAL-19 [o] no screen may imply the product read a whole post
- QUAL-20 [o] the collected unit is a phrase, not a bare noun ← what is recommended is a whole naming (품질좋은 감자탕), and a list of nouns would recommend nothing actionable
- QUAL-21 [o] a phrase list states what is observed about its phrases and never what they improve
- QUAL-22 [o] a phrase list is product-owned data, refreshed by the batch and never editable by an owner ← an edit would fork it away from the refresh
- QUAL-23 [o] the 분야 list is the product's own, taking Naver Blog's 주제 categories as its reference rather than its contract; v1 is 맛집 · 카페 · 국내여행 · 패션·미용 · 상품리뷰 · 육아·결혼 · 반려동물 · 인테리어·DIY · 일상·생각, and each carries a stable ASCII id (restaurant, cafe, domestic_travel, fashion_beauty, product_review, parenting_marriage, pets, interior_diy, daily_life) and the query string its batch sends, which is its name with · replaced by a space ← the names have to be stable identifiers the product controls, and Naver's picker is neither versioned nor published as a list
- QUAL-24 [o] the batch is a product cost and appears in no owner-facing quota
- QUAL-25 [o] Naver is the only platform measured and the only corpus read; a second platform arrives whole, with its own metrics and its own corpus
- QUAL-26 [o] the nouns come from the write pass for both languages and the containment rule follows the post's `content_language`, a post with none being treated as Korean ← legacy content predates the language field and was Korean; product-owned Korean and English stopword lists serve the phrase runs (→QUAL-38)
- QUAL-27 [x] a composite quality score across the metrics
- QUAL-28 [x] a keyword density percentage, a recommended title length, a target character count, alt-text or heading-tag checks, and any 블로그 지수 prediction from views or dwell time
- QUAL-29 [x] measuring the finished draft and re-running a correction pass when a metric is over
- QUAL-30 [x] the owner typing an analysis site's number in by hand
- QUAL-31 [x] reusing 블라이's observed bands ← they were produced by an undisclosed formula against a different numerator
- QUAL-32 [x] a product-authored dictionary of commercial keywords as M1's numerator
- QUAL-33 [x] full-text collection of top-ranked posts
- QUAL-34 [x] any search-demand or 키워드 단가 data, including the 검색광고 API
- QUAL-35 [x] measuring whether a post was cited by a generative answer ← no screen, feed or API is known to report it
- QUAL-36 [o] ② shows this post's own M2, M3 and M4 values against the same bands; M1 appears only in ①'s brief; an account under M2's minimum renders that minimum on ② exactly as the brief does
- QUAL-37 [o] a post's own numbers are shown whether or not any band is crossed ← they are the author's reading of their own draft, not a warning the product raises
- QUAL-38 [o] a phrase is a 2-5 token run appearing in the field's collected titles and descriptions; runs made only of stopwords are dropped and the 50 most frequent survive as that field's list ← a single token recommends nothing and a longer run repeats too rarely to be a pattern
- QUAL-39 [o] recency everywhere means `published_at` descending, so a post republished under a new URL enters the window at its new position
- QUAL-40 [o] a measurement that cannot be computed is absent rather than zero, and an absent value neither passes nor warns
- QUAL-41 [o] a 분야 whose phrase list is empty, before its first batch or on a box without Naver keys, behaves as no 분야 (→GEN-48)
- QUAL-42 [o] the product runs without Naver keys: the batch does not run, every list stays empty, and nothing on screen names the missing keys
- QUAL-43 [o] M3's rule text tells the writer not to keep repeating one noun through the body, varying or dropping the repeats, and to cover in the body what the title names; it names no noun ← M3 is measured inside each post, so no noun from earlier posts belongs in the next one
- QUAL-44 [o] M4's rule text tells the writer to build the body from at least three distinct block types, mixing in whichever of HEADING, LIST and QUOTE the material fits, and to invent nothing the source lacks to fill one
- QUAL-45 [o] every rule text yields to natural writing: it asks for no synonym, cut or block that would read forced ← a rule that makes the post read unnatural costs more than the band it chases

## flow
- measure (post): content revision changes → that post's own metrics computed and stored → ② renders them
- measure (account): URL pasted or cleared → the set of 발행됨 posts changes → aggregate recomputed from their stored rows
- offer: ①'s dock reads the aggregate → a metric over its band renders its checkbox and tooltip, one below its minimum renders that minimum → ticked boxes freeze at enqueue → the write prompt carries their rule text
- phrases: daily batch per 분야 → titles and descriptions → that field's phrase list stored → a post's 분야 freezes its list at enqueue

## constraints
- no screen states or implies an expected exposure gain
- a rule reaches the prompt only through a ticked checkbox; ticking nothing adds no quality-rule bytes to the prompt
- a band, a minimum, a stopword list and a phrase list are data the product may change without a schema change
- schema: `post_measurements` (one row per post, keyed by slug with the revision and measure version it describes, cascading with the post) · `field_phrase_lists` (one row per 분야 with its phrases, corpus size and refresh times)
- config: the M2 run length (8 어절), M1's window (100 titles), M2/M3/M4's window (20 posts), the per-metric minimums, the bands and the batch interval are product-owned settings, not per-account options

## chg
- r3 260924 QUAL-43+ 44+ M3's and M4's rule texts decided · QUAL-45+ every rule text yields to natural writing
- r2 260923 QUAL-41+ 42+ an empty list behaves as no 분야 and the product runs without Naver keys · QUAL-4✎ everything stored per revision→M3 and M4 stored, M2 and the aggregate at read · QUAL-7✎ posts' most frequent noun→the write pass's nouns matched against content titles · QUAL-9✎ counts by the same containment rule, absent with no nouns · QUAL-10✎ minimum 3, photos are IMAGE blocks · QUAL-12✎ four row states · QUAL-14✎ the account-level M2 run · QUAL-16✎ nouns ride the write call · QUAL-23✎ ids and query strings · QUAL-26✎ tokenizers→write-pass nouns with a Korean fallback and two stopword lists · QUAL-36✎ ② renders M2's minimum · schema line for post_measurements and field_phrase_lists · constraint "a byte-identical prompt"→"no quality-rule bytes", since every write answer now carries `nouns`
- r1 260923 initial
