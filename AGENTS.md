# Task workflow

The installed gusta skills govern the workflow. Plan with `/gusta-sketch`, or pass a change request to `/gusta-start`, which records it in the SSOT and tasks and then implements them unattended; without one, `/gusta-start` runs the tasks already planned. It has no mode to choose and commits per unit. The branch finished work lands on and whether it is pushed follow `spec/SETTINGS.md`. Work in the existing `main` checkout; preserve unrelated local changes and keep them outside each unit's commit. Package-managed skill files remain unchanged.

The package FORMAT is `.claude/skills/gusta-create-architecture/assets/FORMAT.md` (`npx @haeram/gusta vibe format --section <name>` prints an excerpt); `spec/FORMAT.md` modifies it (chg keeps only unconsumed revs, a removed feature's decisions are deleted rather than marked [x]). Read notation from the needed sections of `spec/FORMAT.md`; it wins wherever the two differ.

# Verification

Verification commands come from `spec/ssot/ARCH.md`: ARCH-24 for task completion and ARCH-31 for what a push requires. Runnable commands are in `docs/verification.md`.

- Task completion: run the tests added or modified by the task plus existing tests for plausible side effects, and the relevant lint, formatting and build/type checks. Assess the entire task delta and affected consumers; expand to the affected full suite when the impact cannot be bounded. Record the commands and selection rationale in the task result. An empty test selection does not verify a behavioral change.
- Before each push: run the full CI checks and, when the push can trigger `Deploy backend` or `Verify media`, the deployment/media checks. Evidence from an unchanged source, configuration and dependency tree is reused (`vibe check`), so a push that adds only spec documents does not rerun them.
- A local pass does not establish remote deployment success. After each push inspect every triggered workflow for the pushed revision and use its failing job, step and error to diagnose a failure.
