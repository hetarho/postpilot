# IDEATION clip-template-as-preset
> st:converted@260917 | Make the video template a reusable preset instead of a precondition, and let the owner shape captions in the preview

## vision
- [o] Problem: a clip cannot be generated without a template (CLIP-5 `exactly one`, CLIP-25, CLIP-130), so the template became a gate rather than a convenience; a post takes `at most one` with no default (TMPL-1) and the two products drifted apart.
- [o] Target: the owner making a clip, who wants to pick an intro, an outro, the caption styles and a written instruction directly and only later save that set for reuse.
- [o] Core value: design selection and the instruction belong to the project; the template only carries a starting set plus the flow the owner keeps reusing.

## explored
- [o] A generation takes at most one template; a project with none is generated from its own settings and the guide contributes no bytes, matching TMPL-1 and TMPL-12.
- [o] The design selection — intro preset, outro preset, the allowed caption styles and the faces — moves from the template (CLIP-4, CLIP-14, CLIP-111) to the project; a project made from a template starts with the template's values and may change every one of them, the way CLIP-139 already treats pace and accent.
- [o] A template carries an ordered list of named composition stages (식당 소개 · 메뉴 설명 · 식사 · 특징 · 마무리), each a name plus one line, reaching the flow call as guidance in the same position as the template guide.
- [o] Stages never gate: footage that fits no stage is skipped or merged by the writer and the result is still delivered, keeping CLIP-136's "no template structure admits or forbids footage" intact.
- [o] The owner picks which caption styles a project may use and the writer assigns only from that list, so render cost stays bounded and a project keeps one voice.
- [o] The draft preview becomes editable: each caption is drawn over a representative frame of the cut it plays on, and the owner drags its position freely, resizes it and changes its style per caption.
- [o] A dragged caption stops at the design safe area (CDS-9, CDS-13) so a caption can never leave the region a platform leaves visible.
- [o] Contrast below the 4.5:1 floor becomes a preview warning naming the caption rather than a refusal, because the owner now owns the position.
- [x] Required stages that refuse a generation when unmatched ← CLIP-4 records this exact failure: a template that scripted the footage refused every clip whose footage did not fit.
- [x] Keeping the twelve discrete anchors (CDS-12) as the only placement ← the point of the editable preview is free placement, and an anchor grid cannot express it.
- [x] Letting the writer choose freely from every caption style ← sequence-rendered styles cost 34 s per 3.2 s caption, so an unbounded choice makes render time unpredictable.
- [x] Playing the real footage under the editable preview ← it needs the originals in the browser and collides with the 24-hour retention rule (CLIP-21); a cut frame answers position and contrast at a fraction of the cost.

## shape
- flow: create project (template optional) → ① design selection, allowed caption styles, faces, instruction, sources → AI observation → AI assembly → editable preview (position · size · style per caption) → render → ③ finalize.
- v1: template optional with the design selection on the project; named stages as guidance; allowed-style list; editable preview over cut frames with safe-area clamping and contrast warnings.
- later: saving a project's current selection back as a new template; per-stage style hints.

## domains
- [o] CLIP: template becomes optional, design selection moves to the project, composition stages, allowed-style list, editable draft preview →CLIP
- [o] CDS: the caption style set grows past one treatment, free placement inside the safe area, contrast demoted to a warning →CDS
- [x] TMPL: post templates are unchanged ← only the clip side was `exactly one`; TMPL-1 is the shape being copied, not altered.

## open
- [?] How the preview guarantees it matches the renderer: the front end must draw the same SVG the renderer rasterises, and resvg and the browser are known to disagree (feDisplacementMap lands in the wrong place in resvg 0.48.1, and filters need `color-interpolation-filters="sRGB"` to match) — each shipped style needs a both-sides check.
- [?] The ceiling on sequence-rendered captions per project, and whether the ceiling is counted at selection time or at render time.
- [?] What happens to existing templates and projects: their design selection, their anchor-placed captions and their frozen compositions.
