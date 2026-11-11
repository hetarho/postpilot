"""Execute the repository's ARCH-31 runbook as the loop's final gate."""
import fnmatch
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path.cwd()
RUNTIME = Path(__file__).resolve().parent


def run(args, *, cwd=ROOT, env=None):
    print("RUN", " ".join(map(str, args)), flush=True)
    subprocess.run(list(map(str, args)), cwd=cwd, env=env, check=True)


def output(args):
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()


version = (ROOT / ".node-version").read_text().strip().removeprefix("v")
node_bin = RUNTIME / f"node-v{version}-linux-x64" / "bin"
env = dict(os.environ, PATH=f"{node_bin}:{os.environ['PATH']}")
actual = subprocess.check_output([str(node_bin / "node"), "--version"], text=True).strip()
if actual != f"v{version}":
    raise RuntimeError(f"Node pin mismatch: {actual}, expected {version}")

blocks = re.findall(r"```sh\n(.*?)```", (ROOT / "docs/verification.md").read_text(), re.S)
if len(blocks) != 4 or "pnpm test:dev" not in blocks[0] or "--skip-storage-build" not in blocks[3]:
    raise RuntimeError("Verification runbook changed; reselect its commands before pushing")

# Preserve the runbook commands while keeping its disposable venv in this run's runtime.
full_ci = blocks[0].replace("/tmp/postpilot-deploy-checks", str(RUNTIME / "deploy-checks"))
run(["bash", "-c", full_ci], env=env)

base = os.environ.get("HAERAM_DIFF_BASE") or output(["git", "rev-parse", "origin/main"])
changed = output(["git", "diff", "--name-only", base, "HEAD"]).splitlines()
workflow = (ROOT / ".github/workflows/deploy-backend.yml").read_text()
paths = workflow.split("    paths:\n", 1)[1].split("\njobs:", 1)[0]
patterns = re.findall(r"^      - '([^']+)'$", paths, re.M)
if not patterns:
    raise RuntimeError("Deployment path filters could not be read")
media = any(fnmatch.fnmatchcase(name, pattern) for name in changed for pattern in patterns)
media = media or any(name == ".github/workflows/media-verify.yml" or
                     name.startswith("scripts/media-") for name in changed)
if not media:
    print("Deployment/media gates not triggered by this push delta", flush=True)
else:
    # Match the runbook's three Compose invocations with disposable example env files.
    with tempfile.TemporaryDirectory(prefix="compose-", dir=RUNTIME) as temporary:
        target = Path(temporary)
        (target / "deploy/media").mkdir(parents=True)
        for name in ("docker-compose.prod.yml", "docker-compose.media.colocated.yml",
                     "docker-compose.media.remote.yml"):
            shutil.copyfile(ROOT / name, target / name)
        shutil.copyfile(ROOT / ".env.production.example", target / ".env")
        shutil.copyfile(ROOT / "deploy/media/worker.env.example", target / "worker.env")
        for name in ("docker-compose.yml", "worker.env.example"):
            destination = "worker.env" if name == "worker.env.example" else name
            shutil.copyfile(ROOT / "deploy/media" / name, target / "deploy/media" / destination)
        compose_env = dict(env, IMAGE_TAG="validation", MEDIA_WORKER_IMAGE_TAG="validation",
                           API_UPSTREAM="postpilot-api-validation")
        for layout in ("colocated", "remote"):
            run(["docker", "compose", "--env-file", ".env", "--env-file", "worker.env",
                 "-f", "docker-compose.prod.yml", "-f", f"docker-compose.media.{layout}.yml",
                 "config", "--quiet", "--no-env-resolution"], cwd=target, env=compose_env)
        run(["docker", "compose", "--env-file", "worker.env", "-f", "deploy/media/docker-compose.yml",
             "config", "--quiet", "--no-env-resolution"], cwd=target, env=compose_env)
    for block in blocks[2:]:
        run(["bash", "-c", block], env=env)
