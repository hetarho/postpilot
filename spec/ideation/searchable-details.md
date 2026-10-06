# IDEATION searchable-details
> st:open@260926 | Retire the 네이버 검색 API 분야 phrase batch and everything it feeds, find what else raises a post's quality and its Naver search visibility, and show the owner which published posts still draw search readers

## vision
- [o] Problem A: the 분야 phrase batch cannot run and should not: developers.naver.com stopped issuing 검색 API keys on 2026-07-31 (the API moved to NAVER API HUB, free now and metered later), and the 검색 API 특약 in force from 2026-09-07 forbids feeding its results to AI (prompt input, generation, evaluation, exposure), caching them and earning revenue from them; GEN-48 puts the phrases in the write prompt, the batch stores them, and the product is paid
- [o] Problem B: even where it ran, the phrase list aimed at the wrong target: an owner's real 유입 검색어 (about 95 inflows) are almost all a place or a shop joined to a name, a branch, a dish or a price (구리 양촌리, 스시메이진 구리, 옆떡 가리봉점, 청량리 갈비탕, 양촌리오리가격, 이디야 아침 메뉴), and not one is a 분야-wide phrase of the 품질좋은 감자탕 kind
- [o] Problem C: an owner's views settled near 100 and are now falling, and nothing tells them which post stopped drawing search readers: their Naver statistics show search inflow only for the blog as a whole
- [o] three causes look the same on a views graph and need different treatments: a post missing from search (누락), a post still present but ranked lower, and a post holding its rank while fewer searchers click through to it
- Outside context, 2026: AI 브리핑 tops about 20% of 통합검색 queries and shows blogs only as cited sources, and bloggers report 애드포스트 revenue down about 40% (ZDNet, 2026-08-06); Naver Blog MAU fell from 3.13M (2025-09) to 2.88M (2026-08), and Naver says first-hand, expert posts will be shown ahead of indiscriminate AI-generated ones (조선일보, 2026-10-01)
- [o] Core value: more readers arriving from Naver search; reader satisfaction matters but ranks second when the two pull apart
- [o] The product's own writing prompt gets better at search inflow by learning from 유입 검색어 that owners hand over, pooled across blogs
- [?] Target: to be narrowed once the direction is chosen

## explored
- [o] retire the phrase batch and every consumer of it: the stored 분야 phrase lists, the frozen phrase section in the write prompt, the write answer's replacement candidates and ②'s marked spans, and the 상위 노출 단어 사용 guideline preset ← the source is closed to new keys, its terms forbid the use, and real inflow shows the phrases were not what brought readers
- [x] keep the phrase feature and move it to NAVER API HUB ← the 2026-09-07 terms forbid AI input, caching and monetization whatever the endpoint
- [x] store the search results first and send only the stored copy to the model ← the origin of the data, not the path it took, is what the terms name, and storing is itself restricted
- [x] mix Naver phrases with Google-sourced phrases so the list has no single origin ← a Naver-sourced phrase stays Naver data inside a mixed list, and obscuring origin reads worse if challenged
- [x] 검색광고 API keyword volume and related terms ← stays rejected as QUAL-34 decided
- [o] the owner keeps feeding the product their blog's own 유입 검색어, and what it teaches reaches the writer ← first-party data, so no 검색 API terms apply, and it is what actually brought this blog's readers
- [o] the owner hands the 유입 검색어 over as a screenshot of Naver's own statistics screen ← the lowest-effort input an owner can repeat
- [x] fetching 유입 검색어 by API ← Naver publishes no blog-statistics API
- [x] reading the statistics screen automatically in the owner's browser ← the product has no companion any more (PUB-47), and automated access to Naver screens is what its terms restrict
- [o] what the product learns from 유입 검색어 is how a post has to be written to draw search inflow (title shape, where the details stand, which details readers look for), never the keywords themselves ← a keyword belongs to the past post it reached, and carrying it into a new one is off-topic repetition
- [x] reusing an owner's past 유입 검색어 verbatim in new posts ← they name past shops and dishes, so in a new post they are unrelated keywords of the kind Naver's spam rules target
- [o] pooled 유입 검색어 strengthens only the product's own prompt, the code-owned writing rules every account shares ← one owner's screenshot is too thin to personalize from, and the value is in the pattern across blogs
- [x] per-account rules drawn from an owner's own 유입 검색어 ← same reason
- [o] credits for handing over 유입 검색어, paid later rather than at upload ← the delay leaves time to verify before anything is paid
- [o] verification: a keyword counts only when it matches one of the uploader's own 발행됨 posts in the product, only accounts with published posts take part, one reward per month with a cap, and the operator spot-checks before a monthly payout ← only a keyword tied to a post the product wrote teaches how writing draws inflow, so the check and the data's value are the same test
- [x] the server reads Naver search pages to show each post's rank band for its main keywords (1위 / 2~10위 / ~100위 / 그 외), as 포포몬 and 블덱스 do ← not settled as illegal (the Supreme Court found no crime in collecting public data without evading a protective measure, 2022-05-12 2021도1533; a civil court found bulk crawling and republishing a database-right infringement, 잡코리아 v. 사람인 2017), but search.naver.com's robots.txt bars every bot and names AI and RAG, the product is paid and would feed ranks to a model, QUAL-47 already keeps such data out, and one server IP means a block ends the feature while evading the block brings criminal risk
- [x] collect ranks but keep them away from the model ← robots.txt bars every bot, not only AI ones, and the block risk is the same
- [x] an AI analysis written from collected ranks ← falls with the rank collection
- [o] the owner-facing view covers only the product's own 발행됨 posts ← the product already holds their URLs and content, so a finding can be tied to how the product wrote the post
- [x] every post on the blog, fetched from the blog's link ← a post written outside the product cannot teach how the product should write, and the public RSS feed stops at the 50 most recent posts
- [o] per-post search inflow without ranks: each keyword of the blog-wide 유입 검색어 list is matched to the 발행됨 post it names, so the owner sees which posts still draw search readers ← the same match the contribution check already makes, turned toward the owner
- [?] the owner checks a keyword's rank in their own browser from a search link the product gives, and records the band
- [o] the owner sees a per-post inflow view of their 발행됨 posts: which still draw search readers and through which keywords ← Naver shows it one post at a time at best, and the view gives the owner a reason to keep handing data over
- [o] Naver statistics show each post's own 유입경로, as the owner confirmed on 2026-10-02 ← a per-post screenshot ties inflow to its post without guessing
- [?] whether a post's 유입경로 names the search keywords or only the channel (search, neighbours, outside)
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

## shape
- v1: retire the batch, the phrase prompt section, replacement candidates, ②'s spans and the preset / the replacement: not yet chosen

## domains
- QUAL: retire the phrase batch decisions, the `field_phrase_lists` schema line and the phrases flow; M1..M4 measurement stays →QUAL
- GEN: retire the frozen phrase list and the replacement candidates →GEN
- GUIDE: retire the 상위 노출 단어 사용 preset →GUIDE
- POST: retire ②'s replacement spans →POST
- inflow learning and contribution credits: to be placed once the shape settles (QUAL, GEN, QUOTA candidates)
- per-post inflow view for the owner: QUAL candidate, beside the published-post measurement

## open
- reading a screenshot takes a model call while QUAL-16 says a measurement costs no credit: who pays for reading it
- what the per-post view hands the owner beyond the match: a report only, or suggested changes to a published post (locked in the product by POST-74)
- what the owner consents to when handing 유입 검색어 over for pooling
- learning how writing leads to inflow needs each keyword tied to the post it reached; whether a blog-wide screenshot is enough or a per-post one is needed
- whether an owner will keep feeding 유입 검색어, and what makes it low-effort enough to keep happening
