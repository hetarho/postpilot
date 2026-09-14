# REVIEW clip-failure-visibility-260914
> st:converted@260914 | scope:frontend/src/features/inspect-clip-observations, frontend/src/features/edit-clip-project, frontend/src/pages/clip | at:eebe34c | base:ARCH@2

## summary
- two P2s on the same seam: the product records exactly why it refused something and then shows the owner a sentence that cannot be acted on
- both were found by reading production records against the screen an owner actually saw, and both contradict a decision already made (CLIP-86, CLIP-39)

## findings
- F1 [o] P2 `frontend/src/features/inspect-clip-observations/ui/ClipAttemptInspection.tsx:159` `namedExplanation`: bug: it consults only the observation and render check maps, so every `composition_*` and `plan_*` check resolves through the ternary chain into `explanation`, which a present `job.failure` then replaces outright ← CLIP-86 requires the structured failure and its checkpoint TOGETHER, and this is the path that matters most: job `546a0878` recorded `composition_section_order` at cut 2 and the owner saw only "AI 결과 형식을 읽을 수 없어요", with nothing to act on, after three writer retries and 20 analysed sources →T156
- F2 [o] P2 `frontend/src/shared/lib/save-state.ts:27` + `features/edit-clip-project/model/clip-draft-queue.ts:112`: bug: `SAVE_STATUS_LABEL_KEYS.error` is the single key `state.saveRetrying` ("저장하지 못했어요 · 다시 시도 중"), while `terminal(error)` stops the queue for exactly the refusals that will never succeed ← an `invalid_argument` refusal tells the owner to wait for a retry that was already abandoned, and never names the input at fault; the form's own `AppFailureMessage` carries the reason but is mounted only while step ① is (CLIP-36), so moving to ② or ③ leaves the red line as the only account of a permanently unsaved draft (CLIP-39 makes that line the screen's substitute for a save button) →T157

## notes
- not a finding: `ClipStatus` already gives a failing save precedence on the one status line, and `ClipProjectForm` already renders the refusal through `AppFailureMessage`. The gap is what each says, not whether either exists
