# IDEATION searchable-details
> st:open@260926 | Retire the 네이버 검색 API 분야 phrase batch and everything it feeds, and find what else raises a post's quality and its Naver search visibility

## vision
- [o] Problem A: the 분야 phrase batch cannot run and should not: developers.naver.com stopped issuing 검색 API keys on 2026-07-31 (the API moved to NAVER API HUB, free now and metered later), and the 검색 API 특약 in force from 2026-09-07 forbids feeding its results to AI (prompt input, generation, evaluation, exposure), caching them and earning revenue from them; GEN-48 puts the phrases in the write prompt, the batch stores them, and the product is paid
- [o] Problem B: even where it ran, the phrase list aimed at the wrong target: an owner's real 유입 검색어 (about 95 inflows) are almost all a place or a shop joined to a name, a branch, a dish or a price (구리 양촌리, 스시메이진 구리, 옆떡 가리봉점, 청량리 갈비탕, 양촌리오리가격, 이디야 아침 메뉴), and not one is a 분야-wide phrase of the 품질좋은 감자탕 kind
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
- [?] a file export from 크리에이터 어드바이저 instead of a screenshot, if Naver offers one for 유입 검색어
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

## open
- what the owner consents to when handing 유입 검색어 over for pooling
- learning how writing leads to inflow needs each keyword tied to the post it reached; whether a blog-wide screenshot is enough or a per-post one is needed
- whether an owner will keep feeding 유입 검색어, and what makes it low-effort enough to keep happening
