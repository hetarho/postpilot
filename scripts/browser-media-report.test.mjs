import assert from "node:assert/strict";
import {
  mkdtempSync,
  mkdirSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  benchmarkReport,
  fixtureCatalog,
  parseByteRange,
  resolveFixtureFile,
  validateFixtureBundle,
} from "./browser-media-report.mjs";

test("catalog covers ratios, style extremes, long clips, rates and independent audio", () => {
  const catalog = fixtureCatalog(["bold", "neon", "glitch", "ember"]);
  assert.equal(validateFixtureBundle(catalog), catalog);
  assert.equal(catalog.cases.length, 16);
  for (const ratio of ["vertical", "horizontal", "square"])
    for (const style of ["bold", "neon", "glitch", "ember"])
      assert(
        catalog.cases.some(
          (value) => value.ratio === ratio && value.styles.includes(style),
        ),
      );
  assert(catalog.cases.some((value) => value.durationMs === 60_000));
  for (const audio of ["none", "source", "narration", "mixed"])
    assert(catalog.cases.some((value) => value.audio === audio));
  assert.throws(() => fixtureCatalog(["bold", "bold"]));
  assert.throws(() =>
    validateFixtureBundle({ ...catalog, provenance: "uploaded-user-media" }),
  );
  assert.throws(() =>
    validateFixtureBundle({
      ...catalog,
      cases: [catalog.cases[0], catalog.cases[0]],
    }),
  );
});

test("unmeasured phases and unsupported output are not reported as zero-cost successes", () => {
  const fixture = { id: "vertical-bold", durationMs: 15_000 };
  const report = benchmarkReport({
    fixture,
    renderer: "canvas2d-current",
    temperature: "cold",
    environment: { graphics: "software", audioCodec: false },
    result: {
      status: "unsupported",
      phases: { sourceRead: 3 },
      reason: "AAC_UNAVAILABLE",
    },
  });
  assert.equal(report.phases.sourceRead, 3);
  assert.equal(report.phases.upload, null);
  assert.equal(report.output, null);
  assert.equal(report.productionQualified, false);
  assert.throws(() =>
    benchmarkReport({
      fixture,
      renderer: "pixi-webgl",
      temperature: "warm",
      environment: {},
      result: { status: "done", output: { frameCount: 449 } },
    }),
  );
  assert.throws(() =>
    benchmarkReport({
      fixture,
      renderer: "pixi-webgl",
      temperature: "warm",
      environment: {},
      result: { status: "failed", phases: { video: -1 } },
    }),
  );
});

test("fixture serving rejects path traversal, foreign symlinks and invalid ranges", () => {
  const root = mkdtempSync(join(tmpdir(), "browser-media-paths-"));
  try {
    mkdirSync(join(root, "fixtures"));
    writeFileSync(join(root, "fixtures", "source.mp4"), "video");
    writeFileSync(join(root, "outside.json"), "{}");
    symlinkSync(
      join(root, "outside.json"),
      join(root, "fixtures", "foreign.json"),
    );
    assert.equal(
      resolveFixtureFile(join(root, "fixtures"), "source.mp4"),
      join(root, "fixtures", "source.mp4"),
    );
    for (const name of [
      "../outside.json",
      "/outside.json",
      "foreign.json",
      "source.mp4?token=secret",
    ])
      assert.throws(() => resolveFixtureFile(join(root, "fixtures"), name));
    assert.deepEqual(parseByteRange("bytes=2-4", 6), { start: 2, end: 4 });
    assert.deepEqual(parseByteRange("bytes=-2", 6), { start: 4, end: 5 });
    assert.deepEqual(parseByteRange("bytes=3-", 6), { start: 3, end: 5 });
    for (const header of [
      "bytes=6-",
      "bytes=3-2",
      "bytes=-0",
      "bytes=0-1,3-4",
      "bytes=-",
    ])
      assert.throws(() => parseByteRange(header, 6));
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("cold/warm reports preserve verified output identity and distinguish failed work", () => {
  const fixture = {
    id: "vertical-bold",
    ratio: "vertical",
    durationMs: 15_000,
  };
  const output = {
    frameCount: 450,
    durationMs: 15_000,
    width: 1080,
    height: 1920,
    bytes: 1234,
    sha256: "a".repeat(64),
    sampledFrames: 3,
  };
  const report = (temperature, result) =>
    benchmarkReport({
      fixture,
      renderer: "canvas2d-current",
      temperature,
      environment: {},
      result,
    });
  const cold = report("cold", { status: "done", elapsedMs: 20, output });
  const warm = report("warm", { status: "done", elapsedMs: 10, output });
  assert.equal(cold.output.sha256, warm.output.sha256);
  assert.notEqual(cold.temperature, warm.temperature);
  assert.equal(cold.elapsedMs, 20);
  assert.equal(warm.elapsedMs, 10);
  const failed = report("warm", {
    status: "failed",
    elapsedMs: 8,
    reason: "DECODER_FAILED",
  });
  assert.equal(failed.status, "failed");
  assert.equal(failed.output, null);
  for (const mutation of [
    { bytes: 0 },
    { sha256: "" },
    { durationMs: 500 },
    { width: 720 },
    { sampledFrames: 0 },
  ])
    assert.throws(() =>
      report("cold", { status: "done", output: { ...output, ...mutation } }),
    );
});
