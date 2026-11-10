# REVIEW perf-quality-sil5-261110
> st:converted@261110 | scope:frontend tests and frontend production code | at:48a0bca8 | base:ARCH@21

## summary
- Two behavior-preserving fixes approved for the user's self-improve-local invocation until 21:00 KST with a push of the finished part; a setup-cost proposal is limited to a measurement, and one scout claim was refuted.

## findings
- F1 [o] P1 `frontend/src/pages/clip/ui/ClipCampaign.test.tsx:35`: bug: under a broad loaded run the synchronous `필수 해시태그` query fails right after the `필수 키워드` query at line 34 succeeded, while an isolated run passes; a ClipWorkspace update outside act is logged for the same test ← blocks the ARCH-25 full FE gate and therefore every main push →T743
- F2 [o] P3 `frontend/src/features/answer-voice-prompt/ui/VoiceAnswerComposer.tsx:88-96`: bug: Back stays enabled while a photo converts, so leaving the form mid-conversion creates an object URL at line 93 after the unmount cleanup at line 76 already ran; that URL is never revoked and keeps the converted blob until page unload ← memory held per abandoned conversion →T744
- F3 [?] P2 `frontend/src/test/setup.ts:3-5`: every jsdom file imports and initializes the full i18n catalog graph; the full suite reports setup 2167.90 s against tests 1426.57 s (cumulative) ← slows every FE verification; the dominant setup cost is unmeasured, so only a bounded profile is approved in this run
- F4 [x] P2 `frontend/src/entities/clip-plan/ui/ClipSpeechPlayer.tsx:34-71`: bug: cached speech blob survives regeneration ← refuted: the exported wrapper at lines 13-15 keys the inner player by owner, project and assetId, so a regenerated speech remounts with an empty resource

## notes
- Authority: direct user request on 261110 18:18 KST to run self-improve-local for performance, test-code optimization and bug fixes until 21:00 KST, then push only what is finished by then. No standing schedule, paid calls, public contracts, persistence changes or new product policy. Approval is proposal selection, not code-review approval.
- Fixed base 48a0bca835c4039fb3a8d84d5978df81a3b31b07 (main = origin/main, clean). The user pushed it at 18:17:59 KST; its CI and Deploy backend runs were in progress at the start of this run.
- Scope: frontend only. Any backend/** or deploy path triggers Deploy backend and Verify media, whose local parity (image builds, worker smoke, media-release) does not fit before 21:00 and is known to be flaky locally; such proposals wait for a later run.
- Scouts: two read-only sonnet sessions (frontend tests; frontend production code), proposals returned to the orchestrator. Orchestrator inspected ClipCampaign.test.tsx:20-45, /tmp/postpilot-go-ci-261110/frontend-final.log:85-92 and :250-262, ClipSpeechPlayer.tsx:1-80 with ClipSpokenProperties.tsx:95-130, VoiceAnswerComposer.tsx:70-100 and :209-222.
- F1 approval: the remount/collapsed-disclosure explanation is a hypothesis; waiting with findBy would not recover a collapsed disclosure, so reproduction under load is mandatory before any change. Allowed: a test change that waits for the same settled state with unchanged assertions, or removing an unintended product remount with a regression test. Not allowed: weaker assertions, retries, raised timeouts, skips, changed disclosure defaults. Budget 50 min plus independent review.
- F2 approval: guard only the conversion completion (unmounted or superseded pick revokes its new URL and does not set state); preserve conversion, preview, upload, Back and copy behavior. Budget 20 min plus independent review.
- F3 decision: a config/setup change affects every FE test and cannot be validated with a full suite before the 21:00 gate alongside F1/F2, so only a disposable-worktree profile runs now; its numbers decide a later task.
