# Clip release QA — 2026-09-16

CDS-53's owner review, run over rendered clips. The review is manual by nature: this
document is its evidence. Nothing here is answered by an agent — every answer below is
the owner's, recorded verbatim. An empty answer means that pass has not been run yet.

Anything that fails becomes a review finding (`spec/review/`) or an update-ssot request.
It is never fixed silently in this document.

## Clips under review

CDS-70 leaves four selectable region pairs (intro A|B x outro B|E). Three clips cover all
four presets; the pairing is recorded per clip so a later run can repeat it exactly.

| clip | intro | outro | caption | cut rates | file |
|---|---|---|---|---|---|
| 1 | A 크기만 | B 가로선 구분 | bold | 1x · 1x · 0.5x | `tmp/release-qa/clips/qa-clip-1-intro-a-outro-b.mp4` |
| 2 | B 위아래 가로선 | E 점수 강조 | bold | 0.75x · 1.25x · 1x | `tmp/release-qa/clips/qa-clip-2-intro-b-outro-e.mp4` |
| 3 | A 크기만 | E 점수 강조 | bold | 1.5x · 2x · 1x | `tmp/release-qa/clips/qa-clip-3-intro-a-outro-e.mp4` |

All three are 1080x1920, 30 fps, 15.8 s, H.264 + AAC.

**Footage**: synthetic only — three 60 fps originals, each a flat hue crossed by one
white square with its own sine tone, so a cut boundary, a changed rate and which original
is playing are all observable without any recorded material. A 60 fps original is what
makes the slow rates offerable at all (CLIP-99).

**Render path**: `TestReleaseQAViewingClips` in the Docker `media-smoke` target, with
`CLIP_RELEASE_QA=1` and `CLIP_SMOKE_EXPORT` pointed at the directory above. No network,
no model, no stored project — repeat a later run by rebuilding that target.

Every clip carries a disclosure badge, one information pair (label + value) and at least
one caption, so the same questions apply to all three.

## Viewing passes

Each clip is watched twice: once muted, once with its configured source audio. The muted
pass comes first — CDS-53's recall questions are about what the picture alone carries.

### Clip 1 — intro A / outro B

| # | question | muted pass | source-audio pass |
|---|---|---|---|
| 1 | which two facts do you recall without rewinding? | | |
| 2 | is the subject known within 1.5 s? | | |
| 3 | does the copy read as editor-placed rather than auto-dropped? | | |
| 4 | is the tone constant from first cut to end card? | | |
| 5 | is the disclosure legible for its whole interval? | | |

### Clip 2 — intro B / outro E

| # | question | muted pass | source-audio pass |
|---|---|---|---|
| 1 | which two facts do you recall without rewinding? | | |
| 2 | is the subject known within 1.5 s? | | |
| 3 | does the copy read as editor-placed rather than auto-dropped? | | |
| 4 | is the tone constant from first cut to end card? | | |
| 5 | is the disclosure legible for its whole interval? | | |

### Clip 3 — intro A / outro E

| # | question | muted pass | source-audio pass |
|---|---|---|---|
| 1 | which two facts do you recall without rewinding? | | |
| 2 | is the subject known within 1.5 s? | | |
| 3 | does the copy read as editor-placed rather than auto-dropped? | | |
| 4 | is the tone constant from first cut to end card? | | |
| 5 | is the disclosure legible for its whole interval? | | |

## Assembly cases

The cases CDS-53 names, checked across the same three clips. Each must stay coherent and
preview-equivalent, with no unintended audio, temporal gap, stale caption, invented text
or silent repair.

| case | where it appears | result |
|---|---|---|
| same-source split (two cuts from one source) | every clip: cuts 1 and 2 are source 1 split at one point in its own timeline | |
| every rate preset (0.5 / 0.75 / 1 / 1.25 / 1.5 / 2) | clip 1 covers 1x and 0.5x, clip 2 0.75x and 1.25x, clip 3 1.5x and 2x | |
| transition overlap | every clip: a 200 ms fade into cut 2 | |
| transformed caption boundary | captions run 3–7 s and 7–11 s, crossing the cut boundaries at 5.0 s and 9.8 s; in clips 2 and 3 both neighbouring cuts are rate-transformed | |
| an absent optional field | clip 3: the information element carries its label and no value row | |

## Findings

| # | clip | what was seen | where it goes |
|---|---|---|---|

_No findings recorded yet._
