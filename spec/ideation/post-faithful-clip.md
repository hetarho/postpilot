# IDEATION post-faithful-clip
> st:ready@261015 | a clip that tells the finished post's story, cut the way a person would cut it

## vision
- Problem: the app's clips come out as dry descriptions of the footage. The clip writer never sees the visit as the owner told it. On 261015 the 참선주면옥 clip was made by hand from the same 11 sources and the finished post, and it read like a person's short-form review. That one clip is the bar.
- Target: an owner who writes the post for a visit and also wants a clip of it, in either order.
- Core value: a finished post is the clip's story. The post supplies the order, the owner's impressions and the concrete wording, and the footage shows them.

## explored
- [o] Diagnosis of 261015 (the hand-made 참선주면옥 clip vs the app). The clip arc (CLIP-213) already matches the hand-made cut, so the gap is material, not form:
  - Story material. The hand cut followed the post's storyline paragraphs and body. The app's writer gets only the instruction, and here the memo was empty. CLIP-214 carries the title and memo but not the body.
  - Owner impressions. 내 감상은 그대로 forbids any verdict, rating or will to return that the owner did not give. Without the post, 정말 맛있었어요, 4점 and 또 올게요 cannot be written, so captions fall back to describing the footage.
  - Facts the footage cannot show: 아내와 퇴근 후, 평일 저녁 대기 없이, 태블릿 주문, 체험단.
  - Placement. Photo and video files interleave by capture order (31136.mp4 sits between 31135.jpg and 31137.jpg). So each video falls into the post section whose photos surround it.
- [o] A linked post's canonical content reaches every clip writing call as owner-supplied material: its title, body, section order, ratings and stated impressions. Those impressions count as the owner's for 내 감상은 그대로. ← The owner kept and finished that text as their own account of the visit, and only it can supply the verdict and the wording.
- [o] A clip links to at most one post, from either side:
  - From the post: ③'s 이 체험으로 클립 만들기 links the post to the new clip.
  - From the clip: ①'s 글 연결 picks one of the owner's posts that has canonical content. Posts whose title contains the clip's title are listed first. The owner can unlink.

  ← 참선주면옥's clip was created at 11:40, before its post at 11:46, so a link offered only from the post side would have missed it.
- [o] The linked post is read when each attempt starts and frozen with it, as the instruction is (CLIP-69). Editing the post later changes the next attempt, not the current one.
- [o] With a linked post, the storyline (both 스토리라인 먼저 and 바로 만들기) follows the post's section order. Each paragraph shows the videos that belong to that section. A post section with no video is left out of the clip. The owner reviews and edits it in ② as today (CLIP-179). ← This reuses storyline-first rather than adding a second planning path.
- [o] Videos are placed by capture time first: a video shot between the photos of a post section belongs to that section. The analysis then confirms that the content fits. With no usable capture time, the AI matches by content. ← Capture order was exact for 참선주면옥, and content matching alone can confuse similar shots such as the two 국밥 spoonfuls.
- [o] Captions carry the post's own wording, condensed to the caption bounds, rather than describing the footage. The first caption names the place from the post (the 상호). The last one carries the owner's verdict when the post has one. ← 살얼음 육수, 쫄깃한 면발 and 대기 없이 입장 were the parts that made the hand cut read as a person's review.
- [o] A CLIP-213 stage that the post gives no material for is left out rather than filled. Examples: no 아쉬운 점 when the post says there were none, and no 정보 when the post gives none. ← A 20 s clip has no room for filler, and filling such a stage would invent content.
- [x] The storyline alone, without the body ← it loses the concrete wording and the impressions, which are the point.
- [x] AI composition free of the post's order ← that is what produced the dry, shot-list result.
- [x] A link only from post to clip (T692 as specced) ← it misses a clip started before its post, which is what happened here.
- [x] Photos as clip sources ← CLIP-214 keeps them out. Photos still help place the videos in time, but they are not shown.
- [?] Whether a span marked AI가 보탠 내용 in the post's origins counts as the owner's. Recommendation: yes, because the owner finished the post with it. Revisit if the origin labels become reliable.
- [?] Whether the default caption look already gives two lines with one highlighted phrase (the hand cut's white plus yellow) through the existing caption styles and accent. If so, it is a default choice, not a new style.

## shape
- flow: write and finish the post → link it (③'s 이 체험으로 클립 만들기, or the clip's ① 글 연결) → add the videos → 스토리라인 먼저: the post's section order, with videos placed by capture time and confirmed by the analysis → ② review the storyline → 이 스토리로 만들기 → render
- v1:
  - In: a link from both sides; the post's canonical content as material for every writing call; the storyline seeded from the post's order; capture-time placement with a content check; empty stages left out; the 상호 in the hook and the verdict in the outro.
  - Not in: photos as sources, background music, more than one post per clip, re-syncing an attempt already running when the post changes.

## domains
- CLIP:
  - CLIP-214 extended to link the post and carry its content.
  - New: the clip-side 글 연결, and the post as frozen writing material (with CLIP-69 and CLIP-183).
  - The storyline follows the post's order (CLIP-177, CLIP-178, CLIP-181).
  - Capture-time placement.
  - Empty CLIP-213 stages are left out.
  - The 상호 hook and the verdict outro, drawn from the post.
- POST: POST-120 links the post as well as seeding the clip.
- GUIDE: 내 감상은 그대로 counts a linked post's stated impressions as the owner's.
- CDS: the default caption look, only if the existing styles cannot give two lines with one highlighted phrase.

## open
- Do the post's photos keep a usable capture time after upload, or is it stripped? This is a feasibility concern for capture-time placement. The fallback is content matching.
- How much the post material grows the clip's writing prompt, and whether that slows generation the way the voice profile did for posts. Measure before deciding whether the body needs trimming.
- Collision with in-flight tasks: T692 implements CLIP-214 as it stands, and T688 builds the clip arc. The SSOT update should land before either is started, or become a follow-up task.
