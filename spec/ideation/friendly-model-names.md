# IDEATION friendly-model-names
> st:open@261013 | Ordinary users meet models as a grade plus a stable alias (가성비 · 소나무) instead of provider model names and versions; details are opened only on request.

## vision
- Problem: user-facing pickers and the writing-test studio show catalog labels such as `Google: Gemini 2.5 Flash Lite Preview 09-2025`; ordinary bloggers cannot tell what the vendor, family and version strings mean, so the choice reads as technical noise.
- Target: ordinary users choosing an active model, a comparison candidate or a recommendation set; operators keep exact identities.
- Core value: the name a user reads answers "which tier, and which line within it" and stays the same across screens, removals and version upgrades, while the exact model stays reachable for anyone who wants it.

## explored
- [o] Grade-based short names are the primary label on user surfaces; the provider model name/version moves to an on-request detail view ← the owner's proposal; grades (MODEL-57) already exist per purpose and already lead picker labels
- [o] The grade shown before an alias is that purpose's grade ← classification is per purpose, so one model can read 가성비 · 소나무 for writing and 밸런스 · 소나무 for photo analysis
- [o] Operator surfaces (모델 관리, 추천 조합, paste document) keep exact ids/labels and additionally show the short name users see
- [o] A user-facing name, once given, never changes and never moves to an unrelated model ← saved selections and test history carry it; free `:free` routes churn often
- [x] Positional numbering recomputed as 1..N ← removing 가성비2 turns yesterday's 가성비3 into today's 가성비2, so saved selections and history appear to name a different model
- [x] Fixed never-reused numbers ← stable, but churn grows them into names like 가성비145
- [o] Each model carries an alias; the grade is not part of the alias but is shown before it (가성비 · <alias>) ← a fixed name needs something other than an ever-growing number
- [o] An alias is assigned automatically from a fixed word list at registration, and the operator may change it ← no registered model is ever left without a user-facing name and no operator work is required
- [x] Operator-written alias as a mandatory step ← another gate like classification before a model can reach users
- [x] Aliases proposed only by /recomend-models ← models registered by checkbox in 모델 관리 would still lack one
- [o] Aliases are neutral unique words (가성비 · 소나무) that imply no quality, and a retired alias is never given to an unrelated model ← history must not point a past name at a different model
- [x] Shortened real names (제미나이 플래시) ← a new version of the same family collides with the old one and needs its version back
- [x] Letters (A, 알파) ← run out quickly, and reuse confuses history
- [o] One alias per model across every purpose it serves ← the same word in two stages tells the user it is the same model
- [o] A new version of the same line inherits its predecessor's alias (e.g. a 5 → 5.5 upgrade keeps 소나무) ← users follow a line they know rather than a version string
- [o] On succession a saved selection of the predecessor follows to the successor only when the successor has the same grade for that purpose and the account may use it, and the user is told once that the alias moved to a new version; otherwise the selection stays and the user is asked to choose again ← a grade change can change per-post credits, so following unconditionally would be a silent substitution (MODEL-24)
- [x] Always follow regardless of grade ← a user who misses the notice discovers higher per-post credits later
- [x] Never follow ← many users would stay on retired versions
- [o] Handing an alias to a successor is a replacement: the predecessor leaves user lists, and work already started with it finishes on it ← one list never shows the same name twice
- [x] Keep the predecessor as 소나무 (이전 버전) ← more choices and later operator cleanup for a rare side-by-side need
- [o] The operator confirms succession; registration suggests existing models from the same provider with similar names, and /recomend-models may propose succession in the paste document ← names alone cannot separate lines such as Flash and Flash Lite
- [x] Automatic succession by name similarity ← a wrong match would move users' selections to a different line
- [o] History and test results show the alias with the exact version frozen at that time behind 자세히 (MODEL-40) ← after succession the same alias names a newer version
- [o] Every user surface that names a completion model adopts the alias together (active selection, creation pickers, test studio, champion reveal and history, recommendation-set contents) ← one model under two names on different screens defeats recognition
- [?] The automatic alias word list (theme and contents) must be decided; candidate themes were trees, flowers and stars, and speed-suggesting animals were excluded
- [o] Within one grade the basis for choosing is what the product already knows: per-post credits (QUOTA-64) and capability badges such as 영상 ← no operator copy to maintain
- [x] Operator-written one-line traits (긴 글에 강함, 빠름) ← per-model copy upkeep and unfounded quality claims
- [x] Name only ← gives no basis for choosing inside a grade
- [o] Lists show only short names; the chosen model's field offers 자세히, which reveals the exact model name/version, provider, context length and per-post credits
- [x] Page-wide real-name toggle and a separate model guide table ← not chosen for now; one detail place is enough
- [o] Users keep choosing a model, not only a grade ← choosing a grade alone would let an operator reorder silently change the model in use, against the no-silent-substitution rule (MODEL-24)

## shape
- flow: open picker → read `<grade> · <alias>` rows in grade order, each with per-post credits and capability badges → choose → optional 자세히 under the field shows exact model, provider, context and per-post credits
- succession: operator registers a new version → confirms 이어받기 from suggested predecessors → alias moves, predecessor leaves user lists → eligible same-grade selections follow with a one-time notice; others are asked to choose again
- v1: aliases on every user completion-model surface, automatic assignment with operator edit, operator-confirmed succession, per-post credits and capability badges in options, 자세히 under the chosen field / not: page-wide real-name toggle, model guide table, operator-written traits, grade-only selection, automatic succession

## domains
- MODEL: model-wide alias assignment/edit, never-reused aliases, operator-confirmed succession and its effect on saved selections, operator visibility, paste-document round trip, frozen alias+version in test history
- CDS THEME: picker option/detail presentation
- QUOTA: grade/plan copy beside short names (locked grades)

## open
- The automatic alias word list: theme, size and contents (must be decided before SSOT conversion)
- Whether speech profiles (목소리 만들기/대본 읽기, MODEL-83 friendly names) join the alias scheme or keep their own names
