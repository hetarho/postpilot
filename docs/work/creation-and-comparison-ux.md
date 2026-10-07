# Creation and comparison UX implementation

Implement one dependency-ready task at a time in the existing `main` checkout. [STATE](../../spec/STATE.md) records progress; each task's referenced SSOT decisions govern behavior. The planning source is [creation-and-comparison-ux](../../spec/ideation/creation-and-comparison-ux.md). Final navigation arrangement, history naming/placement and visual dimensions remain open under THEME-61.

T622's shared contracts and the T624/T625/T629 implementation are present on `main`. Complete the remaining tasks in this dependency-compatible order:

| Task | Result | Prerequisites |
|---|---|---|
| T623 | Visible location, parent/return navigation, routes and clip creation chrome | T622 T629 |
| T626 | Editable voice materials, accepted profile snapshots and synthetic candidate publication | T622 T625 |
| T630 | Named template/guideline/model states, shared editing and domain publication | T622 T625 |
| T627 | Frozen single-factor inputs and one complete post per contestant | T622 T625 T626 T630 |
| T628 | Durable human tournaments, metering, cancellation, publication receipts and retention | T622 T624 T625 T626 T627 T630 |
| T631 | Complete UX wiring and qualify creation settings and sixteen-entry tests | T622 T623 T624 T625 T626 T627 T628 T629 T630 |

Read STATE and the selected task before implementation. Confirm its prerequisites are complete on `main`, record the start, implement its stated scope and preserve unrelated changes. Run its acceptance and ARCH-24 checks, record commands and selection rationale, complete its task/STATE records and commit on `main` before proceeding.

Use the published T622 contracts and completed prerequisite behavior. T625 supplies shared authoring; T626 supplies accepted voice/material/candidate behavior; T630 supplies template/guideline/provider validation and publication; T627 supplies generation plans; T628 supplies tournament jobs, usage and receipts. T623 registers navigation/routes, and T631 completes the required adapters, RPC registration, global resources and root exports. Preserve existing media behavior where clip UI code is shared.

Post test-output publication follows T624's post contract; provider model adoption follows T630's provider contract. Each target domain commits its mutation and action receipt atomically. T628 recovers from the committed receipt, including after a later manual change.

T631 verifies actual API/store/queue/transport wiring and the complete creation/settings/test experience. A sixteen-entry test produces sixteen validated posts and fifteen human decisions; decisions issue no provider calls. Count separate candidate preparation and common observations explicitly. Fixture success does not establish provider semantic quality, and functional verification does not decide THEME-61's open visual choices.

Current SSOT revisions and pending deltas in STATE remain authoritative; reassess task freshness before implementation. Preserve unrelated browser-media and INFRA work. Full CI and applicable deployment/media checks run once for the final candidate before an authorized push under ARCH-31.
