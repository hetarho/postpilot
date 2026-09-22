import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";

const root = path.resolve(import.meta.dirname, "..");
const deployedGuide = readFileSync(path.join(root, "DEPLOY.md"), "utf8");
const pendingGuide = `
T312 bridge 이미지: migration 0072 → migration 0076
운영 실행과 production purge는 여전히 pending
api report --environment prod --output report.json
api cleanup --report report.json --report-digest digest \\
  --shutdown-inventory shutdown.json --shutdown-digest digest --receipt cleanup.json
Inspect first, then --apply and --verify.
`;

// Execute the real CLI in a disposable repository so negative fixtures cannot alter the
// working tree or mistake a restored publishing runtime for a harmless documentation edit.
function fixture(t, deploy) {
  const directory = mkdtempSync(
    path.join(tmpdir(), "postpilot-retirement-check-"),
  );
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const write = (file, content) => {
    const destination = path.join(directory, file);
    mkdirSync(path.dirname(destination), { recursive: true });
    writeFileSync(destination, content);
  };
  write("scripts/check-publishing-retirement.mjs", "");
  copyFileSync(
    path.join(root, "scripts/check-publishing-retirement.mjs"),
    path.join(directory, "scripts/check-publishing-retirement.mjs"),
  );
  write(
    "frontend/src/app/routes/publishing.ts",
    "const legacyRoute = '/publishing-agents'; redirect({ to: '/posts', replace: true })",
  );
  write("README.md", "Manual export only.");
  write("spec/NARRATIVE.md", "Manual export only.");
  write("DEPLOY.md", deploy);
  return {
    write,
    run: () =>
      spawnSync(process.execPath, ["scripts/check-publishing-retirement.mjs"], {
        cwd: directory,
        encoding: "utf8",
      }),
  };
}

test("accepts the checked-in deployment guide after the prod checkpoint is completed", (t) => {
  const result = fixture(t, deployedGuide).run();
  assert.equal(result.status, 0, result.stderr);
});

test("also accepts a deployment whose operational cleanup is still pending", (t) => {
  const result = fixture(t, pendingGuide).run();
  assert.equal(result.status, 0, result.stderr);
});

for (const required of [
  "T312 bridge 이미지",
  "migration 0072",
  "migration 0076",
  "--report-digest",
  "--shutdown-inventory",
  "--shutdown-digest",
  "--receipt",
  "--verify",
]) {
  test(`rejects a missing bridge prerequisite: ${required}`, (t) => {
    const result = fixture(t, pendingGuide.replaceAll(required, "")).run();
    assert.equal(result.status, 1, result.stdout);
    assert.ok(
      result.stderr.includes(`DEPLOY retirement bridge lost: ${required}`),
      result.stderr,
    );
  });
}

test("completed operational cleanup cannot permit the retired runtime to return", (t) => {
  const repository = fixture(t, deployedGuide);
  repository.write(
    "backend/internal/publishing/service.go",
    "package publishing",
  );
  const result = repository.run();
  assert.equal(result.status, 1, result.stdout);
  assert.match(
    result.stderr,
    /retired path exists: backend\/internal\/publishing/,
  );
});
