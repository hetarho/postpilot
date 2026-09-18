# IDEATION post-quality-and-related-links
> st:open@260918 | Measure what code can count about a blog's own recent posts and offer the owner a rule against it, and let an already-published Naver URL come back as a 추천글 position at the foot of a later post

## vision
- [o] Problem A: the product owns how a post sounds (VOICE), what shape it has (TMPL) and what it must avoid (GUIDE), but nothing measures what the account's posts look like next to each other, which is where an outside scorer and Naver's own spam rules both look.
- [o] Problem B: a verified `blog.naver.com` URL is already frozen and retained (PUB-15, PUB-25) and then never read again, so the account's own published back catalogue cannot appear at the foot of a later post.
- [o] Target: the owner publishing to Naver repeatedly, whose blog is judged on behaviour across posts rather than on one post's prose.
- [o] Core value: the product measures what it can count and hands the owner one switch per finding, so blog-level quality stops being luck without a rule ever being applied behind the owner's back.

## explored
- [o] measurement and rule are two separate things: code counts the metric deterministically and what reaches the model is prompt text ← a model cannot count the keyword ratio of prose it has not written, so a prompt-only rule would be a wish rather than a constraint; the measurement is also what tells the product when to speak at all
- [o] the rule is offered, never imposed: a metric over its band puts a checkbox in ①'s dock whose tooltip states what the metric is, why it appeared and what ticking it changes; only a ticked box adds text to the prompt ← the owner keeps the decision, and a clear box leaves the prompt byte-identical to today's, the property GUIDE-4 and GUIDE-18 have protected since r1
- [o] the measurement is taken over the account's recently published posts, not over the draft ← the checkbox stands in ①'s dock before any content exists, so what it can report is the blog's recent behaviour, which is also the number the owner already compares against the outside site
- [o] v1 measures four things: 제목 도배율, fixed-phrase repetition across posts, per-post morpheme repetition with title-body relevance, and the 분량·구성 set (character count, photo count, structural variety, sentence length)
- [o] every metric is a pass/warn badge carrying where its band came from, never a weighted score ← no published weighting exists for any of them, and a composite number would invent precision the sources do not support
- [x] measuring the finished draft in ② and re-running a correction pass when a metric is over ← it buys accuracy for one more model call and one more credit charge per generation, and cannot promise the second pass lands inside the band either
- [x] the owner typing the analysis site's number in by hand ← it never disagrees with the scorer, but it makes the owner the measuring instrument and goes stale silently
- [o] 제목 도배율 is not a Naver metric but 블라이's own screen label with an undisclosed formula; the denominator is confirmed as the account's last 100 titles and the numerator is not ← the product defines and names its own number on screen rather than claiming parity with a formula nobody publishes
- [o] the single quantitative rule Naver itself publishes is that a keyword repeated twice or more in a title risks a penalty (서치어드바이저 콘텐츠 마크업); everything else Naver states about quality is qualitative ← any band beyond that one is the product's own and is labelled as such
- [o] 유사 문서 is deduplication, not a penalty: Naver's Q&A states that publishing the same text twice does not affect ranking and that the earlier one is treated as the original, with 원본 반영 요청 as the remedy ← no screen may say "유사 문서에 걸리면 저품질"; the real exposure is the separate 도배성 · 대량 생성 spam rule
- [x] keyword density as a percentage, a recommended title length, a target character count, alt-text and heading-tag checks, and any "blog index" prediction from views or dwell time ← no primary source for any of them: Naver states titles have no length limit, Google states length alone is not a ranking factor, Naver Blog's editor emits no h1-h6 and `alt=""` with no input UI, and Naver explicitly denies that reader-side numbers the author cannot control move ranking
- [x] jitter on publish times ← Naver states a perfectly regular publishing interval can read as abuse, but PUB-31 rules out scheduling entirely, so there is nothing to jitter
- [x] stating in every post that it was written with AI ← Naver recommends it and it costs nothing, but it would repeat one identical sentence across every post, feeding the very metric this work adds; an owner who wants it writes it as a 고정 문구
- [o] the 추천글 pool is the account's own verified `blog.naver.com` URLs already kept by PUB-15 and PUB-25, never postpilot URLs ← half of "keep the published link" is already built; what is missing is promoting it from terminal history to material a later run reads
- [o] the count belongs to the position, exactly as it does for a photo (TMPL-38): `count` links stand where the position stands, default 1 ← "1 by default, n when n is asked for" resolves to the attribute the grammar already has, so the template stays the single place a post's shape is decided
- [o] links are selected by shared tags first ← the position exists to send a reader to a related post, and recency alone would staple the same few posts to the foot of everything
- [o] a link already used in the account's last N posts is excluded from the candidate set ← Naver names 기-승-전-링크 as its representative link-spam case, and the feature would otherwise generate exactly that pattern automatically, without the owner ever seeing it happen
- [o] a link renders as the post's title followed by its URL ← Naver asks that a link make clear where it leads, and a titled link is also less likely than a bare address to be auto-expanded into the card that would break PUB-22's fence
- [o] a position matching no eligible post is removed entirely rather than filled with unrelated recent posts ← an unrelated link at the foot of a post is worse than no link
- [o] the model never sees a 추천글 URL, nor a copy token standing for one: code fills the position after the content is validated, at the place the template holds ← the database already owns the verified URL and routing it through a model buys nothing while risking a dead link from one altered character; it is also what keeps this position out of TMPL-37's retired category, since nothing is left for a human or a model to fill
- [x] the model picking which links go in, or writing a per-post introduction for them ← TMPL-16 and GUIDE-18 have held "no model writes, suggests or ranks anything about a template" since r1; an introductory line is authored once as a 고정 문구 above the position (TMPL-18), the construct that already exists for exactly this
- [x] the owner hand-picking links per post ← that is exactly the reserved position TMPL-37 retired, a placeholder to hunt for after pasting; automatic filling is what keeps this position out of that category
- [x] asking for links through the memo or a new option in ①'s dock ← the position already states both where and how many, and a memo phrase would need the model to act on a position it deliberately never sees
- [x] claiming anywhere that 추천글 links raise C-Rank ← the official C-Rank post ties Chain to how other sources consume the content, never to an account's own internal links

## shape
- flow (quality): publish → measure that post's metrics (code, no provider call) → the next run's dock reads the account's recent measurements → a metric outside its band renders its checkbox and tooltip → ticked boxes freeze into the payload at enqueue → the write prompt carries their rule text
- flow (추천글): publish → verified Naver URL retained (already true) → a later post whose template holds a 추천글 position is generated normally, the model unaware of the position → after validation, code resolves `count` links, tag-matched and excluding those used in the last N posts, and inserts each as its title plus URL, or removes the position when nothing is eligible → export and publish carry them
- v1: the four metrics with their bands and their pass/warn badges, the offered checkbox and tooltip in ①'s dock, the ticked rule text frozen at enqueue; the 추천글 position filled by code from the account's published Naver URLs
- not: automatic rewriting when a metric is over, a composite quality score, any model involvement in link selection or link prose, links to anything other than this account's own published posts, a memo or dock route to 추천글, an AI-authorship line in every post

## domains
- [?] QUAL (new): the four metrics, how each is computed, its band and its source label, the per-post measurement record, and the rule text each offers when ticked
- [?] GEN: where a ticked rule sits in the write prompt and how it freezes at enqueue, beside the template brief and the guideline texts
- [?] POST: the checkbox row in ①'s dock and where a post's own measurements are shown
- [o] TMPL: a seventh construct, `<slot kind="related" count="n"/>`, authored on the builder like a photo position and removed at fill time when nothing is eligible
- [o] PUB: the verified Naver URL promoted from terminal history to a readable pool, queried by tag and by recent use

## open
- [?] the 도배율 numerator: the most frequent noun across the last 100 titles, or a dictionary of commercial keywords; observed bands are 14% and 18% 양호 and 26% 주의, with no published 위험 case, so the product picks its own bands and says on screen that they are its own
- [?] N, the number of recent posts a link is excluded for, and whether it is counted in posts or in days
- [?] what the dock shows before the account has published enough posts to measure: no checkbox at all, or a stated minimum
- [?] whether the metric rules are code constants (like GEN-16's grounding constraint and GEN-17's `NaturalnessBaseline`) or rows the owner can edit
- [?] what a position does when the eligible pool is smaller than `count`: render fewer links, or remove the position as the empty case does
- [?] whether a 고정 문구 authored above a 추천글 position disappears with the position when it resolves to nothing: the grammar relates no two sibling blocks, so today the line would stay and announce links that are not there
- [?] the block a link becomes: GEN-1's flat Block types have no LINK, and the four exports (EXPORT) plus the reading view each have to render it
- [?] whether revision may rewrite a 추천글 block: GEN-41 filters only IMAGE and VIDEO against a fresh snapshot, so a revise pass would currently be free to reword or drop a link
- [?] a link whose Naver post the owner later deleted on Naver: the product cannot observe a deletion it did not perform, so the pool would keep offering a dead URL
- [?] same-image reuse across posts, which Naver's Q&A names directly ("동일하게 사용된 사진의 비중이 높으면 유사문서가 될 수 있습니다"): low exposure here because each post carries its own freshly attached photos, but it is the one thing a photo-driven product should know it is not measuring
- [?] RISK, unverified: PUB-22's fence requires the exact semantic body sequence with no inserted text or image, and PUB-36 inserts text at a caret; if Naver's editor turns a pasted address into a link card, the fence observes a block it never froze and fails the publish closed — a titled link reduces but does not remove this, so it has to be settled by an actual publish before v1 ships rather than reasoned about
