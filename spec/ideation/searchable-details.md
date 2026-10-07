# IDEATION searchable-details
> st:open@261007 | Help an owner's posts attract search readers by relating search demand to observed inflow, with repeatable low-effort input and a writing policy beyond tag selection

## vision
- [o] Core value: more readers arriving at the owner's posts from search; two separate questions matter: how much a query is searched, and whether that query brings readers to the owner's posts
- [o] The owner's observed queries join a place or shop to a branch, dish, price or other specific detail (구리 양촌리, 스시메이진 구리, 양촌리오리가격, 이디야 아침 메뉴); broad 분야 phrases did not describe these arrivals
- [o] Available owner data: daily, weekly and monthly inflow queries and search-channel shares, including post-level statistics; the earlier blog-wide-only assumption is superseded
- [o] Repeated input burden is part of the product problem: useful analysis that owners will not supply data for fails the goal
- [o] Tag selection is an interim improvement; the intended policy must connect the post's material, title, body and tags to observed search intent
- [o] Current foundation: the 분야 phrase pipeline is retired; commit `995bcee9` and GEN-50 prioritize grounded entity and area/topic tags but read no demand data
- [?] Demand, visibility and click choice may each affect inflow; diagnosing their contribution requires different evidence, and an inflow decrease alone cannot identify a cause
- [?] Target: existing Naver bloggers with some searchable published content; narrow by topic and traffic level after choosing the first user benefit
- [?] Product-wide learning, account-specific writing guidance or both: reopen the earlier pooled-only choice before settling the replacement

## explored
- [o] retire the phrase batch and every consumer of it: the stored 분야 phrase lists, the frozen phrase section in the write prompt, the write answer's replacement candidates and ②'s marked spans, and the 상위 노출 단어 사용 guideline preset ← the source is closed to new keys, its terms forbid the use, and real inflow shows the phrases were not what brought readers
- [x] keep the phrase feature and move it to NAVER API HUB ← the 2026-09-07 terms forbid AI input, caching and monetization whatever the endpoint
- [x] store the search results first and send only the stored copy to the model ← the origin of the data, not the path it took, is what the terms name, and storing is itself restricted
- [x] mix Naver phrases with Google-sourced phrases so the list has no single origin ← a Naver-sourced phrase stays Naver data inside a mixed list, and obscuring origin reads worse if challenged
- [?] Search-demand sources, including the search-ad keyword tool: reopen the product question because demand is now an explicit goal; QUAL-34 remains unchanged until a separate SSOT decision
  - no API, third-party supplier or reuse permission is selected in this ideation
  - search-ad competition and ad-click metrics do not measure organic blog ranking difficulty or organic click-through
- [o] Owner-provided inflow evidence should inform how the product writes; it describes actual arrivals to the owner's blog
  - [?] user provision does not by itself settle provider-data reuse, AI processing or cross-account reuse rights; check the selected source's conditions before adoption
- [o] the owner hands the 유입 검색어 over as a screenshot of Naver's own statistics screen ← the lowest-effort input an owner can repeat
- [x] fetching 유입 검색어 by API ← Naver publishes no blog-statistics API
- [x] reading the statistics screen automatically in the owner's browser ← the product has no companion any more (PUB-47), and automated access to Naver screens is what its terms restrict
- [o] what the product learns from 유입 검색어 is how a post has to be written to draw search inflow (title shape, where the details stand, which details readers look for), never the keywords themselves ← a keyword belongs to the past post it reached, and carrying it into a new one is off-topic repetition
- [x] reusing an owner's past 유입 검색어 verbatim in new posts ← they name past shops and dishes, so in a new post they are unrelated keywords of the kind Naver's spam rules target
- [?] Pooled-only learning of the product's shared writing rules: reopened on 261007 alongside account-specific guidance ← the earlier decision treated one screenshot as too thin for personalization
- [?] Per-account guidance from the owner's evidence: reopened on 261007 ← post-linked evidence and repeated observations may support narrower advice; one screenshot still cannot establish a writing rule's effect
- [o] credits for handing over 유입 검색어, paid later rather than at upload ← the delay leaves time to verify before anything is paid
- [o] verification: a keyword counts only when it matches one of the uploader's own 발행됨 posts in the product, only accounts with published posts take part, one reward per month with a cap, and the operator spot-checks before a monthly payout ← only a keyword tied to a post the product wrote teaches how writing draws inflow, so the check and the data's value are the same test
- [x] the server reads Naver search pages to show each post's rank band for its main keywords (1위 / 2~10위 / ~100위 / 그 외), as 포포몬 and 블덱스 do ← not settled as illegal (the Supreme Court found no crime in collecting public data without evading a protective measure, 2022-05-12 2021도1533; a civil court found bulk crawling and republishing a database-right infringement, 잡코리아 v. 사람인 2017), but search.naver.com's robots.txt bars every bot and names AI and RAG, the product is paid and would feed ranks to a model, QUAL-47 already keeps such data out, and one server IP means a block ends the feature while evading the block brings criminal risk
- [x] collect ranks but keep them away from the model ← robots.txt bars every bot, not only AI ones, and the block risk is the same
- [x] an AI analysis written from server-collected search ranks ← falls with the rejected server-side rank collection
- [o] the owner-facing view covers only the product's own 발행됨 posts ← the product already holds their URLs and content, so a finding can be tied to how the product wrote the post
- [x] every post on the blog, fetched from the blog's link ← a post written outside the product cannot teach how the product should write, and the public RSS feed stops at the 50 most recent posts
- [?] Per-post inflow inferred by matching a blog-wide query to a post's names: reopen attribution quality ← a matching name identifies a candidate but does not prove that the reader opened that post
- [?] the owner checks a keyword's rank in their own browser from a search link the product gives, and records the band
- [o] the owner sees a per-post inflow view of their 발행됨 posts: which still draw search readers and through which keywords ← Naver shows it one post at a time at best, and the view gives the owner a reason to keep handing data over
- [o] Naver statistics show each post's own 유입경로, as the owner confirmed on 2026-10-02 ← a per-post screenshot ties inflow to its post without guessing
- [?] Actual blog-statistics screens: confirm which post-level screens show queries, counts or only channel shares; the owner confirms post-level statistics exist
- [?] fallback for the blog-wide list only, how a keyword finds its post: the keyword is tied to the 발행됨 post whose title, body or template answers carry the name, branch, area or menu it names; a keyword fitting several posts or none needs a rule (unattributed, candidates shown, or the owner picks)
- [?] the statistics screen's 조회수 순위 (the most-viewed posts by day, week or month, which Naver Blog statistics does show) handed over beside the 유입 검색어 list, so a post's readers are counted rather than inferred and a keyword's candidate posts can be cross-checked against it
- [?] give the writer more varied search keywords ← the product takes no target keyword (GEN-50), and reusing past 유입 검색어 is dropped above
- [?] conflict: QUAL's constraint that no screen states or implies an expected exposure gain, against a per-post view that suggests what to change
- [x] a file export from 블로그 통계 or 크리에이터 어드바이저 instead of a screenshot ← no download is documented for either screen as of 2026-10-02, and third-party extensions exist to fill that gap (조회수 순위 툴킷's CSV, 데이터랩툴즈 헬퍼's CSV); Naver's only export, 글 저장, saves posts as PDF (100 posts a file, 20 files) with no statistics
- [?] the owner selects the statistics table and pastes it as text instead of a screenshot ← reading pasted text needs no model call (QUAL-16), if those screens render their tables as selectable text
- [?] reopen reading the owner's own statistics through a browser extension on their login, which 블덱스 ships (extension updated 2026-07-05, about 3,000 users: 유입 검색어 and 유입 경로 ranks, popular posts, an AI monthly report with next month's posts and a 4-week publishing plan; free shows the latest report only, premium is KRW 47,000 per 30 days; the extension came after 블덱스, once the most-used 블로그 지수 site, ended its index service on 2025-12-04 because it could no longer read the values from Naver, so the extension is new and unproven) ← dropped above for PUB-47 and Naver's terms on automated access; a competitor shipping it changes neither
- [?] 크리에이터 어드바이저's trend screen (popular 유입 검색어 within the blog's 주제) handed over the same way, so a new post has material beyond the blog's own past; its own terms still to check
- [?] the post's own material (template answers, memo) states the searched-for details (name, branch, area, menu, price) plainly and early
- [?] Naver's published ranking principles (C-Rank topic consistency, D.I.A.+ first-hand experience and completeness) as code-owned rules beside GEN-49
- [?] Owner-provided Creator Advisor screens as richer evidence, without server-side search crawling
  - official [query statistics](https://help.naver.com/service/23038/contents/14625?lang=ko&osType=MOBILE) describes exposure, inflow, their ratio, average exposure position, overall search-count trends and the owner's reached posts
  - official [post statistics](https://help.naver.com/service/23038/contents/14605?lang=ko&osType=MOBILE) describes per-post exposure/inflow, average exposure position and query-specific trends
  - official [search exposure analysis](https://help.naver.com/service/23038/contents/14624?lang=ko&osType=MOBILE) normalizes exposure/inflow trends to a period maximum of 100 and limits the included search services; keyword-level data can aggregate several posts
  - official [query trends](https://help.naver.com/service/23038/contents/14623?lang=ko&osType=MOBILE) covers the top 20 inflow queries; [competition comparison](https://help.naver.com/service/23038/contents/14622?lang=ko&osType=MOBILE) suppresses queries with fewer than five inflows
  - these are documented capabilities, not verified availability or extractability in this owner's account; absent or suppressed observations are not zero
- [?] Demand evidence must name what it measures
  | source | supported interpretation | limitation |
  |---|---|---|
  | Owner's inflow list | Queries observed bringing readers to this owner | Does not measure all searches or undiscovered demand |
  | Creator Advisor query detail | Overall search trend alongside this owner's exposure and inflow | Verify actual units and thresholds; a trend is not automatically a monthly search count |
  | [DataLab search trends](https://developers.naver.com/docs/serviceapi/datalab/search/search.md) | Relative search activity for the stated query group and period | Maximum normalized to 100; independently normalized reports are not a common volume scale |
  | [Search-ad keyword tool](https://ads.naver.com/help/faq/1406) | Monthly search counts and downloadable keyword table | Separate account/access effort and reuse conditions; current QUAL-34 excludes demand data |
  | Creator Advisor topic-popular inflow queries | Queries that send readers to content in the selected topic | Inflow popularity differs from overall query search volume |
- [?] Minimum information to receive per observation: source, channel/account, period start/end and day/week/month unit, blog/post scope, query as shown, value/unit and share denominator
  - post identity is required only for a post-scoped observation; accept a visible URL or owner-confirmed identity
  - receive absolute inflows, total inflows, exposure, views and demand only when the source supplies them; keep unavailable values absent
  - preserve capture date, provisional/completed period, screenshot/table coverage and whether the source limits visible rows
  - link to the actual published text/revision when available; the product's generated draft may differ from the published post
- [?] Ratios retain their source denominator; never calculate search inflows as blog views multiplied by an inflow share
  - [Blog inflow analysis](https://help.naver.com/service/5593/contents/15330?lang=ko&osType=COMMONOS) defines path shares over total inflows
  - [Blog views](https://help.naver.com/service/5593/contents/15323?lang=ko&osType=PC) include non-post pages, so blog views need not equal the sum of post views
  - derive a count only from a compatible reported total and denominator, and label a rounded-share result as an estimate
- [?] Compare consistent periods, scope and search channels; daily/weekly/monthly observations are alternative views of overlapping events, not additive input
  - repeated uploads of the same observation must not create additional inflows
  - compare complete periods; normalize by days or observed post age only when meaningful and label the adjustment
  - [Update periods](https://help.naver.com/service/5593/contents/10576?osType=COMMONOS) warns that pre-update statistics may appear as zero; do not learn from provisional zeroes
  - retention differs: daily up to three months, weekly 15 weeks and monthly 26 months; requiring all granularities raises input burden without supplying independent evidence
- [?] Attribute counts to a post only from post-scoped evidence, a native query-to-post breakdown or explicit owner confirmation
  - lexical/title matching may suggest candidates; ambiguous or unmapped queries remain blog-level evidence
  - do not split a blog-level count equally among matching posts or copy it to each post
  - preserve distinct branches, places and query intents; whitespace normalization must not silently combine different targets
- [?] Progressive input proposal: one completed monthly period, one or two overview screens, then an optional query/post detail only for a finding the owner wants to investigate
  - suggest monthly input, with optional weekly follow-up; daily input is for a specific investigation rather than the default habit
  - support image upload/paste as previously adopted; evaluate copied table text where selectable; native exports are optional only for sources that actually provide them
  - identify source, period and scope from the input; ask only for missing or uncertain information rather than requiring a data-entry form
  - give a useful result from a single upload; a second period unlocks changes over time, rather than gating the first result
  - give the owner one immediate action and evidence for it; full-blog completeness, every post's screenshot and contribution verification are not prerequisites for initial feedback
  - [?] one or two screens may not contain both query data and compatible totals; test this burden target against actual screens
- [?] Three acquisition approaches to compare
  | approach | owner effort | value | unresolved concern |
  |---|---|---|---|
  | Monthly overview first | Low recurring effort | Query/intent summary and next-post guidance | Limited per-post attribution and demand evidence |
  | Overview plus selected detail | Moderate only when requested | Better diagnosis for a query/post the owner cares about | Repeated navigation may undermine retention |
  | Authorized automatic source | Low recurring effort after setup | More regular and complete observations | No supported integration established; prior extension/crawling rejection remains in place |
- [?] Owner-facing findings should state evidence and the action it supports
  | observed pattern | useful next step | inference limit |
  | Repeated area + entity + price/menu inflows | Ask for relevant missing factual details before the next write | Does not prove tags or a title pattern caused arrivals |
  | Search share rises while inflow count falls | Show counts and channel context beside the share | A rising share alone is not growth |
  | Overall query demand and owner inflow both decline | Consider topic timing or adjacent material already available | Correlation suggests a demand change, not a sole cause |
  | Demand is steady/rising while owner exposure falls | Investigate visibility and content freshness | No automatic omission/ranking diagnosis |
  | Exposure persists while compatible inflow/exposure ratio falls | Review whether the title and actual answer match the query | Position, search layout and other changes may also affect clicks |
  | Little evidence or only a truncated query list | Show observed intent and suggest what additional detail would help | Missing queries and low counts are not failures |
- [?] One search-intent policy across title, body and tags
  - eligibility first: a candidate query must fit the current post's supported material; search volume cannot make an unrelated query eligible
  - distinguish entity-seeking, comparison, price, access, menu and other intents; keep one primary intent and a few supported adjacent intents
  - title identifies the subject and main answer; the body actually answers it with supplied facts; tags summarize supported subjects and combinations
  - when a useful price/access/detail is missing, ask the owner or omit the claim; do not invent it from old posts or popularity
  - compare demand and this owner's observed strength among eligible candidates when evidence exists; otherwise explain the suggestion from relevance without an invented volume
  - a past query may be relevant again when the current material actually covers the same entity and intent; history alone does not make it relevant
  - treat tag choices as one expression of the policy; no evidence currently isolates their effect on ranking or inflow
- [?] Apply observed intent to material collection before writing: suggest a relevant missing menu/price/access question or template field, rather than only adjusting the final title/tags ← writing cannot supply a factual answer the owner never provided
  - topic/season demand may suggest an adjacent post only when the owner has matching experience/material; query discovery is a separate value from formatting an existing draft
- [?] Product-wide processing and personalized prompts are different layers, not mutually exclusive outputs
  | direction | owner value | product value | risk |
  |---|---|---|---|
  | Pooled product-wide rules first | Delayed benefit shared by everyone | Cross-account patterns and common defaults | Contribution effort before benefit; topic/traffic/sample biases |
  | Personalized prompt only | Immediate account-specific writing guidance | Limited reproducible learning if facts/evidence are discarded | Sparse data may become an overconfident or stale rule |
  | Personal evidence first, optional pooling later | Immediate findings and guidance, then broader improvements | Retained structured observations and validated common patterns | More product scope; requires a clear boundary for secondary reuse |
- [?] Recommended sequence: extract a small amount of comparable owner evidence, return an actionable finding, then offer a preview of account guidance before considering pooled learning
  - keep observed queries/counts separate from interpretations and writing instructions, so later corrections and refreshes do not depend on a prose prompt alone
  - example conditional instruction: for a restaurant post with observed price/menu intent, identify the branch and present supplied menu/price facts clearly; past restaurant names cannot supply facts about a different restaurant
  - candidate account guidance is evidence-linked, date-bounded, topic-specific and owner-editable; uncertainty weakens the instruction rather than producing confident personalization
  - sparse evidence uses common grounded-writing defaults and descriptive observations, without a strong new account rule
  - product-wide learning should examine reusable intent/detail patterns within comparable topics, not pool raw keyword rankings or let large blogs dominate
  - cross-account reuse is a separate undecided choice from processing for this owner's result; raw screenshots need not be retained indefinitely to retain a confirmed observation
- [?] Validation: measure input completion and repeat submission alongside the writing outcome
  - pilot a monthly overview with optional detail; observe time spent navigating/capturing, extraction corrections and whether owners find the immediate action useful
  - proposed usability target: the usual overview submission takes about one minute; validate rather than assume
  - evaluate guidance on repeated comparable published posts, with publication age, topic, demand and channel context; use search arrivals as the main outcome when actual compatible counts exist
  - observed before/after changes do not establish that a prompt, title or tag caused them; require stronger repeated/comparative evidence before promoting a common writing rule
  - keep the existing monthly contribution-credit idea separate from whether the owner receives useful feedback; rewards can distort what gets submitted

## shape
- [o] Current foundation: the phrase pipeline and consumers are retired; grounded tag selection remains under GEN-50
- [?] Proposed core flow: owner provides a completed monthly overview → source/period/scope and uncertain values confirmed → observed queries and intents summarized → one actionable finding → optional query/post detail → preview relevant guidance for the next write → later comparable observation checks the outcome
- [?] Proposed first benefit: choose better title/content/tag emphasis for the next post, with a small evidence report; existing-post diagnosis is an alternative priority awaiting the owner
- [?] Proposed v1 includes partial screenshots, explicit missing evidence, exact attribution when supplied and conditional account guidance; demand information appears only when a suitable source is supplied/adopted
- [?] Broader collection, automatic ingestion, pooled common-rule promotion, contribution payout changes and published-post editing remain unselected; do not require them to deliver initial owner value

## domains
- QUAL: retire the phrase batch decisions, the `field_phrase_lists` schema line and the phrases flow; M1..M4 measurement stays →QUAL
- GEN: retire the frozen phrase list and the replacement candidates →GEN
- GUIDE: retire the 상위 노출 단어 사용 preset →GUIDE
- POST: retire ②'s replacement spans →POST
- QUAL or a separate SEARCH candidate: demand versus inflow evidence, source/period/scope interpretation, attribution, uncertainty and owner findings; must resolve current QUAL-16/QUAL-34 and exposure-claim constraints before conversion
- GEN and GUIDE candidates: one grounded intent policy, account-guidance preview/adoption and relevance gating across title/body/tags
- QUOTA candidate: optional extraction cost and previously discussed contribution credits; neither pricing nor reward changes are decided
- AUTH candidate: account-local processing versus optional cross-account contribution, retention and deletion choices
- POST candidate: published identity/text evidence and future-generation guidance; editing a published post requires a separate policy decision

## open
- reading a screenshot takes a model call while QUAL-16 says a measurement costs no credit: who pays for reading it
- what the per-post view hands the owner beyond the match: a report only, or suggested changes to a published post (locked in the product by POST-74)
- what the owner consents to when handing 유입 검색어 over for pooling
- learning how writing leads to inflow needs each keyword tied to the post it reached; whether a blog-wide screenshot is enough or a per-post one is needed
- whether an owner will keep feeding 유입 검색어, and what makes it low-effort enough to keep happening
- first user benefit: next-post guidance, existing-post diagnosis or equal priority; an optional question was sent on 261007 and remains unanswered
- acceptable input habit: monthly one/two screens, weekly selected details or automatic-only; an optional question was sent on 261007 and remains unanswered
- inspect an actual owner statistics flow to establish screenshot coverage, selectable text, per-post identity and count/share/trend units; official help does not establish capture effort
- whether Creator Advisor gives enough demand context for v1, or monthly search-count evidence is essential; excluded search-demand policy remains current until updated
- minimum evidence and refresh interval for an account instruction; avoid arbitrary thresholds inferred from one successful query
- whether the first personalized result is an editable reusable guideline, per-post suggestions, an exported prompt or a combination
- which findings warrant optional detail, and whether repeat submission provides enough new value without reminders/rewards
- whether automatic ingestion is a hard adoption requirement; no integration or technical mechanism is chosen here
- intended use conditions for each external data source, and account-local versus cross-account reuse consent, deletion and screenshot retention
- how to test improvement in comparable posts without mistaking seasonality, publishing frequency, blog reputation or search-layout changes for a writing-policy effect
