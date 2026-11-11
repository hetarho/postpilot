# IDEATION related-post-links
> st:open@261111 | 추천글: a template position that code fills with links to the owner's own related published posts — deferred from post-quality-and-related-links with its open questions

## vision
- Problem: a reader who finishes a post has no path to the owner's related posts unless the owner adds the links by hand after every export.
- Target: the owner publishing to Naver Blog repeatedly, whose earlier posts are 발행됨 with their pasted URL.
- Core value: the template states where and how many related links stand, and code fills them from the owner's own published posts without the model ever seeing a URL.

## explored
- [o] directions recorded before the owner deferred 추천글 on 260923 (ideation/done/post-quality-and-related-links):
  - the pool is links the owner explicitly saved off a finished post, drawn from the account's own `blog.naver.com` URLs (the URL that makes a post 발행됨), never postpilot URLs
  - `count` belongs to the position, default 1, as it does for a photo position
  - links are selected by shared tags first; a link already used in the account's last N posts is excluded ← Naver names 기-승-전-링크 as its representative link-spam case
  - a link renders as the post's title followed by its URL; a position matching no eligible post is removed rather than filled with unrelated posts
  - code fills the position after the content is validated; the model never sees a 추천글 URL or a token standing for one
- [x] the model picking links or writing a per-post introduction, the owner hand-picking links per post, asking for links through the memo or ①'s dock, and claiming 추천글 links raise C-Rank ← reasons recorded in the source document
- [?] when 추천글 returns: the owner deferred it to follow 발행됨, which the product now has; today TMPL-37 [o] keeps no `place` or `link` position, and a link is written as a 고정 문구

## shape
- flow: publish → the owner pastes the URL and the post becomes 발행됨 → a later post whose template holds a 추천글 position is generated normally, the model unaware of the position → after validation, code resolves `count` links, tag-matched and excluding those used in the last N posts, and inserts each as its title plus URL, or removes the position when nothing is eligible → export and publish carry them

## domains
- [?] TMPL: a seventh construct, `<slot kind="related" count="n"/>`, authored on the builder like a photo position and removed at fill time when nothing is eligible
- [?] POST: the link pool is POST's pasted 발행됨 URL, queried by tag and by recent use (PUB retains no URL to promote)

## open
- N, the number of recent posts a link is excluded for, and whether it is counted in posts or in days
- what a position does when the eligible pool is smaller than `count`: render fewer links, or remove the position as the empty case does
- whether a 고정 문구 authored above a 추천글 position disappears with the position when it resolves to nothing: the grammar relates no two sibling blocks, so today the line would stay and announce links that are not there
- the block a link becomes: GEN-1's flat Block types have no LINK, and the export (EXPORT) plus the reading view each have to render it
- whether revision may rewrite a 추천글 block: GEN-41 filters only IMAGE and VIDEO against a fresh snapshot, so a revise pass would currently be free to reword or drop a link
- a link whose Naver post the owner later deleted on Naver: the product cannot observe a deletion it did not perform, so the pool would keep offering a dead URL
- RISK, unverified: Naver's editor may turn a pasted address into a link card, making the published post differ from the export the product produced; it is settled by pasting once rather than by reasoning
