import { createHash } from "node:crypto";
import { readFileSync, realpathSync } from "node:fs";
import { isAbsolute, relative, resolve, sep } from "node:path";

export const browserMediaRenderers = [
  "canvas2d-current",
  "canvas2d-optimized",
  "pixi-webgl",
  "remotion-web-experiment",
];

export const browserMediaPhases = [
  "sourceRead",
  "nativeAssetPreparation",
  "serverSamplingWait",
  "layoutAssets",
  "video",
  "audio",
  "mux",
  "upload",
];

export function digestFile(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

export function fixtureCatalog(styles) {
  if (!styles.length || new Set(styles).size !== styles.length)
    throw new Error("The fixture style registry must be nonempty and unique");
  const cases = [];
  for (const ratio of ["vertical", "horizontal", "square"])
    for (const style of styles)
      cases.push({
        id: `${ratio}-${style}`,
        ratio,
        durationMs: 15_000,
        styles: [style],
        audio: "none",
        nativeFrames: ["entrance", "settled", "effect-extreme", "exit"],
      });
  for (const [id, durationMs, audio] of [
    ["all-styles-60s", 60_000, "none"],
    ["rates-and-transitions", 15_000, "source"],
    ["narration-only", 15_000, "narration"],
    ["source-and-narration", 15_000, "mixed"],
  ])
    cases.push({ id, ratio: "vertical", durationMs, styles, audio });
  return { version: 1, provenance: "synthetic-only", frameRate: 30, cases };
}

export function validateFixtureBundle(bundle) {
  if (
    bundle.version !== 1 ||
    bundle.provenance !== "synthetic-only" ||
    !bundle.cases?.length
  )
    throw new Error("Unknown or unauthorized browser fixture bundle");
  const seen = new Set();
  for (const fixture of bundle.cases) {
    if (!/^[a-z0-9-]+$/.test(fixture.id) || seen.has(fixture.id))
      throw new Error("Invalid or duplicate fixture identity");
    seen.add(fixture.id);
    if (!["vertical", "horizontal", "square"].includes(fixture.ratio))
      throw new Error("Unknown fixture ratio");
    if (
      !Number.isInteger(fixture.durationMs) ||
      fixture.durationMs < 15_000 ||
      fixture.durationMs > 60_000
    )
      throw new Error("Fixture duration exceeds the output contract");
    if (fixture.prepared) {
      if (
        !fixture.plan ||
        !fixture.source?.file ||
        !/^[a-f0-9]{64}$/.test(fixture.source.fingerprint)
      )
        throw new Error("Prepared fixture lacks a frozen source or plan");
      if (fixture.plan.durationMs !== fixture.durationMs)
        throw new Error(
          "Fixture plan does not match its declared output duration",
        );
    }
  }
  return bundle;
}

/** Only named fixture artifacts are served, including after resolving symlinks. */
export function resolveFixtureFile(root, name) {
  if (
    !/^(?:[a-zA-Z0-9_-]+\/)*[a-zA-Z0-9_.-]+\.(json|png|mp4|mp3|m4a|wav|mjs)$/.test(
      name,
    )
  )
    throw new Error("Invalid fixture artifact path");
  const base = realpathSync(root);
  const path = realpathSync(resolve(base, name));
  const inside = relative(base, path);
  if (
    !inside ||
    inside.startsWith(`..${sep}`) ||
    inside === ".." ||
    isAbsolute(inside)
  )
    throw new Error("Fixture artifact escapes its root");
  return path;
}

export function parseByteRange(header, size) {
  if (!header) return null;
  const match = /^bytes=(\d*)-(\d*)$/.exec(header);
  if (
    !match ||
    (!match[1] && !match[2]) ||
    !Number.isSafeInteger(size) ||
    size <= 0
  )
    throw new Error("Invalid artifact byte range");
  const suffix = !match[1];
  const count = Number(match[2]);
  const start = suffix ? Math.max(0, size - count) : Number(match[1]);
  const end = suffix || !match[2] ? size - 1 : Math.min(size - 1, count);
  if (
    !Number.isSafeInteger(start) ||
    !Number.isSafeInteger(end) ||
    start < 0 ||
    start > end ||
    start >= size ||
    (suffix && count <= 0)
  )
    throw new Error("Unsatisfiable artifact byte range");
  return { start, end };
}

export function benchmarkReport({
  fixture,
  renderer,
  temperature,
  environment,
  result,
}) {
  if (
    !browserMediaRenderers.includes(renderer) ||
    !["cold", "warm"].includes(temperature)
  )
    throw new Error("Unknown benchmark execution");
  if (
    ![
      "done",
      "unsupported",
      "failed",
      "not-implemented",
      "missing-fixture",
    ].includes(result.status)
  )
    throw new Error("Unknown benchmark outcome");
  const phases = Object.fromEntries(
    browserMediaPhases.map((name) => [name, result.phases?.[name] ?? null]),
  );
  for (const value of Object.values(phases))
    if (value !== null && (!Number.isFinite(value) || value < 0))
      throw new Error("Invalid phase measurement");
  if (
    result.elapsedMs != null &&
    (!Number.isFinite(result.elapsedMs) || result.elapsedMs < 0)
  )
    throw new Error("Invalid elapsed measurement");
  if (result.status === "done") {
    const output = result.output;
    const [width, height] =
      fixture.ratio === "horizontal"
        ? [1920, 1080]
        : fixture.ratio === "square"
          ? [1080, 1080]
          : [1080, 1920];
    if (
      !output ||
      output.frameCount !== Math.round((fixture.durationMs * 30) / 1000) ||
      output.width !== width ||
      output.height !== height ||
      !Number.isFinite(output.durationMs) ||
      Math.abs(output.durationMs - fixture.durationMs) > 1 ||
      !Number.isSafeInteger(output.bytes) ||
      output.bytes <= 0 ||
      output.bytes > 128 * 1024 * 1024 ||
      !/^[a-f0-9]{64}$/.test(output.sha256) ||
      !Number.isInteger(output.sampledFrames) ||
      output.sampledFrames < 3
    )
      throw new Error("Successful benchmark has no complete measured output");
  }
  return {
    version: 1,
    fixture: fixture.id,
    renderer,
    temperature,
    environment,
    status: result.status,
    reason: result.reason ?? null,
    elapsedMs: result.elapsedMs ?? null,
    phases,
    videoMeasurements: result.videoMeasurements ?? null,
    peakResources: result.peakResources ?? null,
    output: result.output ?? null,
    productionQualified: false,
    limits: [
      "Observed wall-clock phases may overlap; submission time is not hardware execution time.",
      "Synthetic/local runs do not establish representative PC GPU speed or AI/voice quality.",
      "Private upload and production server sampling are not executed by this harness.",
    ],
  };
}
