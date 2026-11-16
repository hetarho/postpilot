# REPORT 261116-tonenav
> mode:autopilot | host:ihaelam-ui-MacBookPro | from:2026-11-16T14:01Z | to:2026-11-16T14:07Z | push:main

## done
- Lightweight navigation fix: restrict the unmade-voice redirect to that voice's own detail subtree. A departing VoiceLayout no longer overrides another destination while the next route loads.
- Added regression coverage for all five primary navigation links from the method chooser. Existing direct-entry redirection, paste/chat retention, analysis and default-selection coverage remains in the affected suite.
- Set SETTINGS agents to codex,claude through vibe settings; both providers remain available.

## decided
- No new product decision or SSOT revision: restore THEME-26/38 navigation while preserving VOICE-54's unmade-voice learning entry. The change stays within the lightweight path.

## parked
- None in the requested implementation.

## checks
- Baseline: 0715db3bba661f48f62e5bdd57527c348774d814; Node 24.18.0 and pnpm 10.30.3.
- Red reproduction: clicking 만들기 left the settled route at /voices/voice-default/materials instead of /create before the fix.
- Affected Vitest suite: 4 files, 91 tests passed after the fix (PrepareWritingVoice, VoiceAnalysisPage, VoiceMaterialsPage, navigation).
- TypeScript, scoped ESLint, scoped Prettier and Gusta spec lint: passed through vibe check --tier task. Existing spec warnings are unrelated.
- Independent read-only review: Claude Opus 5.5, no findings; no duplicate test execution.
- Browser observation: not-run; this session has no computer-use tool. Primary regression evidence uses the real router and application shell in jsdom; no automated E2E was added or replaced.
- Shared integration and remote CI: pending the fixed-candidate gate at implementation handoff; preserve the final runtime receipt with the session artifacts.

## next
- Submit ready, run the shared fixed-candidate gate once, then land/push under SETTINGS and inspect CI for the actual pushed SHA.
- Review whether the navigation acceptance criteria omit a destination or expect the wrong behavior; manual QA is not assigned to the owner.
