import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";

const root = path.resolve(import.meta.dirname, "..");

// Execute the real CLI in a disposable repository so negative fixtures cannot alter the
// working tree or mistake a restored publishing runtime for a harmless documentation edit.
function fixture(t) {
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
  return {
    write,
    run: () =>
      spawnSync(process.execPath, ["scripts/check-publishing-retirement.mjs"], {
        cwd: directory,
        encoding: "utf8",
      }),
  };
}

test("accepts a repository without the retired runtime", (t) => {
  const result = fixture(t).run();
  assert.equal(result.status, 0, result.stderr);
});

test("rejects a restored publishing runtime", (t) => {
  const repository = fixture(t);
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
