# Creation and comparison UX session bundles

The planning source is [creation-and-comparison-ux](../../spec/ideation/creation-and-comparison-ux.md); current behavior contracts are the linked SSOTs in each task. Functional requirements are taskable, while final navigation arrangement, history naming/placement and visual dimensions remain open under THEME-61. T622 is integrated. Remaining bundles use ARCH@19 after rebasing the group onto current main; the interrupted T625 attempt was released with its partial SQL work preserved for reassignment. Continue one implementation bundle at a time, with independent review and local integration before the next claim.

Each bundle is exactly one T task and one atomic CLI claim. Its acceptance and implementation notes contain its internal milestones. This avoids claiming a fictitious range of tasks when the installed CLI only reserves one task at a time.

| Bundle | Task | Ownership | Integrated prerequisites |
|---|---|---|---|
| B01 | [T622](../../spec/tasks/done/T622.ux-test-contract-foundation.md) | Shared proto/migrations/contracts/generated API, common navigation/workspace primitives and baseline port accommodations | None |
| B02 | [T623](../../spec/tasks/T623.contextual-navigation-and-clip-return.md) | Shell/routes/location/parent/return and clip creation chrome | T622 T629 |
| B03 | [T624](../../spec/tasks/T624.roomy-writing-and-operational-history.md) | Post domain/history projection, spacious editor, compact post/clip history and post deletion return | T622 |
| B04 | [T625](../../spec/tasks/T625.durable-shared-authoring-and-candidates.md) | Authoring backend/entity/core UI, shared AI/direct draft and seed-free candidates | T622 |
| B05 | [T626](../../spec/tasks/T626.editable-voice-materials-and-accepted-snapshots.md) | Voice backend/entity/materials/learning/style factories and voice hosts | T622 T625 |
| B06 | [T627](../../spec/tasks/T627.single-factor-writing-test-snapshots.md) | Generation snapshot/prompt/plan/full-post pipelines | T622 T625 T626 T630 |
| B07 | [T628](../../spec/tasks/T628.durable-human-writing-tournaments.md) | Entire test aggregate/store/runner/RPC/credit bridge/receipts/compatibility | T622 T624 T625 T626 T627 T630 |
| B08 | [T629](../../spec/tasks/T629.unified-writing-test-experience.md) | Test entity/features/widgets/pages/actors and retained comparison readers | T622 |
| B09 | [T630](../../spec/tasks/T630.named-setting-hosts-and-domain-publication.md) | Template/guideline/provider publication and setting hosts/model selection UI | T622 T625 |
| B10 | [T631](../../spec/tasks/T631.creation-tests-integration-qualification.md) | Final cmd/api/routes/resources/root exports and integrated browser/regression proof | All B01–B09 |

T622 must be integrated first. T624/T625/T629 then have separate code ownership and can start together. T625 unlocks voice/settings hosts; T629 unlocks route registration. Domain implementations then unlock preparation and durable execution. T631 is the final integration owner. Ten worker slots are a capacity limit; dependencies and touches decide the actual runnable count, and an idle slot is normal.

The tests UI can develop against the frozen T622 contract and accurate deterministic transports before T628 is implemented. T631 must test the actual integrated handlers; fixture success alone never proves provider semantic quality.

## Start the planning work group

The coordinator first commits this plan, the ten tasks and related SSOT/STATE changes on its planning branch. Exclude unrelated changes, especially the existing searchable-details ideation. Shared STATE ownership must be staged deliberately rather than committing every existing local change.

Then start one common work group with up to ten workers:

```sh
npx haeram-spec-creator work start creation-comparison-ux --workspace auto --workers 10 --reviewers 1 --max-pending 10 --json
```

The returned planning path is the single documentation/numbering/integration workspace. Separate worker worktrees are based on its committed plan. Do not create ten unrelated parent work groups or copy dirty files into workers.

## Claim an unassigned bundle

Use this instruction in a fresh session:

> Read docs/work/creation-and-comparison-ux.md and the shared work board. Select one task only from T622 through T631 whose prerequisites are integrated and which has no active assignment. Atomically claim that T number in work group creation-comparison-ux with this session's unique owner label and a new worktree. If the claim fails because another session claimed it, refresh the board and choose another eligible task. Implement only that bundle in the returned workspace. If none is eligible, report the actual dependency/ownership wait without claiming unrelated tasks.

Read the current board:

```sh
npx haeram-spec-creator work board --work creation-comparison-ux --json
```

T622 is already integrated. Select an eligible remaining bundle from the live board; for example, claim T625 only when the board shows no active assignment:

```sh
npx haeram-spec-creator work claim T625 --work creation-comparison-ux --owner session-01 --workspace new --json
```

After T622 is integrated, other sessions use the same command with their chosen eligible T number and own label. The CLI performs the final atomic duplicate/dependency/touches checks. Move to the returned `workspace`; the task remains todo in planning STATE until reviewed integration, while the runtime board records doing/blocked/ready.

Do not use unrestricted `claim-next` for this scope: the repository also has unrelated browser-media tasks, and the installed CLI has no task-range filter. Board inspection alone does not reserve anything; only a successful explicit claim authorizes ownership.

## Ownership and handoff

- Shared proto/global migration numbering/generated API/common UI are T622-owned first and T631-owned for final integration. Other workers consume the integrated contracts and report contract gaps instead of editing shared files.
- The entire authoring core belongs to T625; all voice domain/source/factory/UI belongs to T626. Template/guideline setting hosts and domain publication belong to T630. These workers exchange published contracts, never edits to each other's files.
- Generation belongs to T627; the full experiment backend belongs to T628. Do not split bracket/store/publication/settlement into competing sessions.
- T623 owns route registration/navigation; T624 owns post editor and operational history; T629 owns test page exports. T631 alone completes shared wiring, global resource assembly and final cross-bundle edits after approvals.
- Domain-specific queries and their sqlc outputs stay with that domain's worker; shared generator scripts/proto outputs do not. The final generator audit runs in T631.
- `touches` locks must also respect unrelated work groups. In particular T623's clip workspace can overlap existing T600; serialize actual file ownership rather than pretending the new UX depends on completing the whole media pipeline.
- Workers update only their own task acceptance/result and runtime heartbeat, never STATE/SSOT or another task. Contract changes return to the coordinator; active task contracts are not changed under a running worker.

## Submit, review and integrate

Use the returned attempt ID for heartbeat/submission. Run the task's appropriate ARCH verification before submission and pass its actual verification commands to `work submit`.

The independent reviewer uses `review-task`; the coordinator integrates only approved submitted commits with `work integrate`. Ready/submitted work does not satisfy a dependency: only integrated done work opens successors. A changed parent commit requires renewed review under the installed CLI.

Release/recovery occurs only after the original worker has stopped, preserving its files and commits. No timeout-based automatic takeover, direct runtime JSON edits, shared-folder parallel writes, remote push or deployment is part of bundle claiming.

## Consumed policy revisions

ARCH@19, CLIP@59, EDIT@3, GEN@25, GUIDE@16, LANG@8, MODEL@34, POST@35, QUOTA@38, THEME@28, TMPL@24, VIDEO@7, VOICE@14. Current SSOT change logs are cleared after task allocation under FORMAT; decisions and their per-task references remain authoritative. Unrelated INFRA pending work is not consumed.

ARCH-5 is consumed as documentation alignment with the already-present authoring context (no standalone code change). THEME-61 remains an open visual review, not an implementation decision. Removed ranking/check/template-helper policies are represented by the current compatibility/retirement contracts in T626/T628/T630.

Post test-output publication is T624-owned, provider model-adoption publication is T630-owned, and T628 depends on both. Each target domain commits its mutation and action receipt atomically; the experiment receipt is recoverable from that committed receipt, including after a later manual change.

ARCH@19 changes verification stages only. Remaining todo bundles reference impact-selected completion checks and record their selection rationale; final qualification retains its explicit integrated coverage. T622 keeps its original reviewed verification record, and the rebase preserves its code tree. Full CI and applicable deployment/media gates run before an authorized push.
