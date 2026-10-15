# Task workflow

The installed haeram-spec-creator skills govern the workflow. Start a session with `/start-vibe` and follow its mode, routing and per-unit commit and push to the STATE `cfg push` branch. Work in the existing `main` checkout; preserve unrelated local changes and keep them outside each unit's commit. Package-managed skill files remain unchanged.

# Verification

Verification commands come from `spec/ssot/ARCH.md`: ARCH-24 for task completion and ARCH-31 for what a push requires. Runnable commands are in `docs/verification.md`.

- Task completion: run the tests added or modified by the task plus existing tests for plausible side effects, and the relevant lint, formatting and build/type checks. Assess the entire task delta and affected consumers; expand to the affected full suite when the impact cannot be bounded. Record the commands and selection rationale in the task result. An empty test selection does not verify a behavioral change.
- Before each push: run the full CI checks and, when the push can trigger `Deploy backend` or `Verify media`, the deployment/media checks. Evidence from an unchanged source, configuration and dependency tree is reused (`vibe check`), so a push that adds only spec documents does not rerun them.
- A local pass does not establish remote deployment success. After each push inspect every triggered workflow for the pushed revision and use its failing job, step and error to diagnose a failure.
