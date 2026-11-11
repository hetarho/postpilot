# IDEATION clip-source-observation-visibility
> st:converted@260912 | Make clip sources easier to browse and AI observations easier to understand

## vision
- [o] Problem: vertical clip source players are hard to browse and AI observation results are hidden.
- [o] Target: people adding videos and generating clips.
- [o] Core value: connect source footage, recorded observations and actual cut usage.

## explored
- [o] Horizontal compact source thumbnails with one selected local preview.
- [o] Read-only recorded summaries and expandable time ranges, events, subjects, speech and quality.
- [o] Actual used ranges and cut numbers from the saved plan, labelled separately when the result has not caught up.
- [o] Existing page progress and failures remain visible outside the horizontal strip.
- [o] Retained observations stay readable without source playback; older observations remain labelled during failed or running regeneration.
- [x] Observation editing, source exclusions, partial streaming, per-source retries and mandatory review ← first scope is read-only inspection without changing generation.

## shape
- flow: select videos → browse source strip → generate → choose retained source → inspect summary and time ranges → see saved-plan cut usage.
- v1: clip source browsing and successful retained observations; source playback requires matching local footage.

## domains
- [o] CLIP: horizontal sources and retained observation inspection →CLIP
- [x] THEME: new shared design rules ← existing THEME-25 and THEME-29 cover the pattern.

## open
- -
