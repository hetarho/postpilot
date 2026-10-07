# Verification

Read `spec/STATE.md` and the verification decisions in `spec/ssot/ARCH.md` before working. The stage-specific policy is ARCH-24 and ARCH-31; runnable pre-push checks are in `docs/verification.md`.

- At task completion, submission and individual task integration, run the tests added or modified by the task plus existing tests for plausible side effects. Assess the entire task delta and affected consumers; expand to the affected full suite when the impact cannot be bounded. Also run relevant lint, formatting and build/type checks.
- Record the commands and selection rationale in the task result. An empty test selection does not verify a behavioral change.
- Run the full CI checks and applicable backend deployment/media checks before push, once for the final candidate. Do not automatically repeat this gate after every task.
- Interpret installed skills' instructions to reproduce CI/CD locally according to these two stages. Package-managed skill files remain unchanged.
- A local pass does not establish remote deployment success. Inspect every triggered workflow for the pushed revision and use its failing job, step and error to diagnose a failure.
