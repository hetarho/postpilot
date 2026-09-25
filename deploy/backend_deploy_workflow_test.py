"""The deploy workflow must still run the image smoke gates somewhere.

ARCH-38 moved the media and clip-input smokes off the deploy's critical path. That is a
change in WHEN they block, not in WHETHER they run — but nothing else in the repository
can tell the two apart, because deleting the gate outright also makes the deploy fast and
the run green. These checks pin the half that must survive: some job still builds the
`production` target (the one whose COPY lines depend on the smoke stages), and that job's
failure still fails the run.

They stay true after the closed-beta restoration too, when `build` goes back to building
`production` itself and the rollout waits for the smoke job again.
"""

import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / ".github" / "workflows" / "deploy-backend.yml"

# The Dockerfile stage the smoke gates hang off: `production` is `runtime` plus two
# `COPY --from=<smoke>` lines, so building it is what forces the smokes to run.
GATED_TARGET = "production"


def build_steps(job):
    """Every docker/build-push-action step in a job, whatever version it pins."""
    for step in job.get("steps", []):
        if str(step.get("uses", "")).startswith("docker/build-push-action"):
            yield step


class SmokeGateSurvives(unittest.TestCase):
    def setUp(self):
        self.jobs = yaml.safe_load(WORKFLOW.read_text())["jobs"]
        self.gating = [
            (name, job, step)
            for name, job in self.jobs.items()
            for step in build_steps(job)
            if (step.get("with") or {}).get("target") == GATED_TARGET
        ]

    def test_rollout_initializes_the_default_worker_explicitly(self):
        steps = self.jobs['rollout']['steps']
        scripts = [step.get('with', {}).get('script', '') for step in steps]
        self.assertTrue(any('sh deploy/backend-rollout.sh --init-worker' in script for script in scripts))
        self.assertFalse(any('sh deploy/backend-rollout.sh --bootstrap' in script for script in scripts))
        synced = [step.get('with', {}).get('source', '') for step in steps]
        self.assertTrue(any('deploy/media/worker.env.example' in source for source in synced))

    def test_some_job_builds_the_gated_target(self):
        self.assertTrue(
            self.gating,
            f"no job builds the `{GATED_TARGET}` target, so the media smokes never run: "
            "the gate was removed, not moved beside the deploy (ARCH-38)",
        )

    def test_the_gating_job_is_unconditional(self):
        """A gate behind an `if:` is a gate someone can switch off without noticing."""
        for name, job, step in self.gating:
            self.assertNotIn("if", job, f"job `{name}` runs the smoke gate conditionally")
            self.assertNotIn("if", step, f"the smoke gate step in `{name}` is conditional")

    def test_the_gating_failure_is_not_ignored(self):
        for name, job, step in self.gating:
            self.assertFalse(
                job.get("continue-on-error"),
                f"job `{name}` runs the smoke gate but its failure does not fail the run",
            )
            self.assertFalse(
                step.get("continue-on-error"),
                f"the smoke gate step in `{name}` cannot fail the run",
            )


if __name__ == "__main__":
    unittest.main()
