# QUAL published-post measurement and 분야 phrases
> r1 | Measure what code can count about a post while it can still be changed and across the account's published Naver posts, offer one rule per finding, and collect each 분야's top-ranking phrases from the official search API.

## decisions
- QUAL-1 [o] QUAL owns the two observations the product makes outside a single post: how an account's published posts look next to each other, and which phrases rank for a 분야 on Naver
- QUAL-2 [o] measurement has two layers: a post's own numbers are computed for any post that has content, while the account aggregate counts 발행됨 posts only ← a number that changes what a later post is told must come from posts that actually went up, but the author needs this post's own numbers while there is still time to act on them
- QUAL-3 [o] a post's own measurement is computed for its current content revision and recomputed when that revision changes; the aggregate is recomputed when a URL is pasted or cleared (→POST-73 →POST-75)
- QUAL-4 [o] each post's measurement is stored against the revision it describes and the aggregate is derived from the stored rows of 발행됨 posts ← per-post numbers are on screen throughout ②, so they are computed once per revision rather than on every read
- QUAL-5 [o] four metrics ship, each a pass/warn badge with its own minimum published count, never a weighted score ← no published weighting exists for any of them
- QUAL-6 [o] every band is the product's own and every screen carrying one says so ← the only quantitative rule Naver publishes is that a keyword repeated twice or more in a title risks a penalty
- QUAL-7 [o] M1 제목 도배율 is an account metric with no per-post value: the share of the account's last 100 발행됨 titles containing its single most frequent noun; minimum 10 published posts ← a saturation over one title is not a quantity
- QUAL-8 [o] M2 글 간 고정 문구: per post, the share of its characters standing inside a run of 8 or more consecutive 어절 that also appears verbatim in one of the account's last 20 발행됨 posts; the account value is the median over those posts; minimum 3
- QUAL-9 [o] M3 글 안 반복과 제목 관련성: per post, the most frequent noun's share of all noun occurrences and the share of the title's nouns appearing in the body; the account value is each median over the last 20; minimum 1
- QUAL-10 [o] M4 분량·구성: per post the character count, photo count, distinct `Block` types used and average sentence length (in characters for Korean, in words for English), of which only the count of distinct block types carries a band ← Naver publishes no target length, so the other three are context shown beside the badge rather than something to pass or fail
- QUAL-11 [o] the bands: M1 warns above 30%, M2 above a 10% median, M3 above an 8% repetition share or below 50% title relevance, and M4 at a median of 2 or fewer distinct block types
- QUAL-12 [o] a metric below its minimum renders a line naming that minimum, not an absent control ← a control that is simply missing cannot be told apart from one that broke
- QUAL-13 [o] each metric owns one rule text, a code-owned constant a 지침 may override
- QUAL-14 [o] M1's and M2's rule texts name the measured phrase itself rather than instructing against overlap in the abstract ← a model acts on a named string reliably and on "겹치는 표현" barely at all
- QUAL-15 [o] a phrase is named by M1 only while it stands in the account's own recent titles, so an account with nothing published bans nothing
- QUAL-16 [o] a measurement calls no provider and costs no credit
- QUAL-17 [o] the 분야 phrase batch runs about once a day per 분야 against the official 네이버 검색 API (`/v1/search/blog.json`, `sort=sim`), taking 300 results per field as three pages of 100, product-side and never as a per-owner live call ← no per-owner key is needed and the 25,000/day quota stays inside the product's control
- QUAL-18 [o] the corpus is the returned `title` and `description` only ← Naver's robots.txt and terms forbid crawling blog.naver.com and name the Open API as the sanctioned route
- QUAL-19 [o] no screen may imply the product read a whole post
- QUAL-20 [o] the collected unit is a phrase, not a bare noun ← what is recommended is a whole naming (품질좋은 감자탕), and a list of nouns would recommend nothing actionable
- QUAL-21 [o] a phrase list states what is observed about its phrases and never what they improve
- QUAL-22 [o] a phrase list is product-owned data, refreshed by the batch and never editable by an owner ← an edit would fork it away from the refresh
- QUAL-23 [o] the 분야 list is the product's own, taking Naver Blog's 주제 categories as its reference rather than its contract; v1 is 맛집 · 카페 · 국내여행 · 패션·미용 · 상품리뷰 · 육아·결혼 · 반려동물 · 인테리어·DIY · 일상·생각, and each carries the query string its batch sends ← the names have to be stable identifiers the product controls, and Naver's picker is neither versioned nor published as a list
- QUAL-24 [o] the batch is a product cost and appears in no owner-facing quota
- QUAL-25 [o] Naver is the only platform measured and the only corpus read; a second platform arrives whole, with its own metrics and its own corpus
- QUAL-26 [o] measurement follows the post's `content_language`: Korean counts nouns and 어절, English counts whitespace-separated words minus a product-owned stopword list, and an account holding both tokenizes each post in its own language before the frequencies are summed ← LANG-20 already draws this line for voice analysis
- QUAL-27 [x] a composite quality score across the metrics
- QUAL-28 [x] a keyword density percentage, a recommended title length, a target character count, alt-text or heading-tag checks, and any 블로그 지수 prediction from views or dwell time
- QUAL-29 [x] measuring the finished draft and re-running a correction pass when a metric is over
- QUAL-30 [x] the owner typing an analysis site's number in by hand
- QUAL-31 [x] reusing 블라이's observed bands ← they were produced by an undisclosed formula against a different numerator
- QUAL-32 [x] a product-authored dictionary of commercial keywords as M1's numerator
- QUAL-33 [x] full-text collection of top-ranked posts
- QUAL-34 [x] any search-demand or 키워드 단가 data, including the 검색광고 API
- QUAL-35 [x] measuring whether a post was cited by a generative answer ← no screen, feed or API is known to report it
- QUAL-36 [o] ② shows this post's own M2, M3 and M4 values against the same bands; M1 appears only in ①'s brief
- QUAL-37 [o] a post's own numbers are shown whether or not any band is crossed ← they are the author's reading of their own draft, not a warning the product raises
- QUAL-38 [o] a phrase is a 2-5 token run appearing in the field's collected titles and descriptions; runs made only of stopwords are dropped and the 50 most frequent survive as that field's list ← a single token recommends nothing and a longer run repeats too rarely to be a pattern
- QUAL-39 [o] recency everywhere means `published_at` descending, so a post republished under a new URL enters the window at its new position
- QUAL-40 [o] a measurement that cannot be computed is absent rather than zero, and an absent value neither passes nor warns

## flow
- measure (post): content revision changes → that post's own metrics computed and stored → ② renders them
- measure (account): URL pasted or cleared → the set of 발행됨 posts changes → aggregate recomputed from their stored rows
- offer: ①'s dock reads the aggregate → a metric over its band renders its checkbox and tooltip, one below its minimum renders that minimum → ticked boxes freeze at enqueue → the write prompt carries their rule text
- phrases: daily batch per 분야 → titles and descriptions → that field's phrase list stored → a post's 분야 freezes its list at enqueue

## constraints
- no screen states or implies an expected exposure gain
- a rule reaches the prompt only through a ticked checkbox; an account that ticks nothing sends a byte-identical prompt
- a band, a minimum, a stopword list and a phrase list are data the product may change without a schema change
- config: the M2 run length (8 어절), M1's window (100 titles), M2/M3/M4's window (20 posts), the per-metric minimums, the bands and the batch interval are product-owned settings, not per-account options

## chg
- r1 260923 initial
