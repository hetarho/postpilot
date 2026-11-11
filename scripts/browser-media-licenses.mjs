import { createHash } from "node:crypto";
import { existsSync, readFileSync, realpathSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const topLevel = [
  "mediabunny",
  "pixi.js",
  "pixi-filters",
  "@resvg/resvg-wasm",
  "@soundtouchjs/core",
];
const sha = (bytes) => createHash("sha256").update(bytes).digest("hex");

export function repositoryURL(value) {
  const source = typeof value === "string" ? value : value?.url;
  if (!source) return null;
  return source
    .replace(/^git\+/, "")
    .replace(/^git@github\.com:/, "https://github.com/")
    .replace(/\.git$/, "");
}

export function licenseObligation(license) {
  if (
    license === "MIT" ||
    license === "ISC" ||
    license === "BSD-2-Clause" ||
    license === "BSD-3-Clause"
  )
    return "Preserve applicable copyright, license and disclaimer notices.";
  if (license === "MPL-2.0")
    return "Preserve notices and provide access to the covered source, including modifications, for distributed JS/WASM.";
  if (license === "Apache-2.0")
    return "Preserve license/notices and identify modifications where applicable.";
  return "Review the exact license expression and bundled dependencies before distribution.";
}

function packageDirectory(name, from) {
  from = realpathSync(from);
  for (
    let directory = from;
    directory !== dirname(directory);
    directory = dirname(directory)
  ) {
    const candidate = resolve(directory, "node_modules", name, "package.json");
    if (
      existsSync(candidate) &&
      JSON.parse(readFileSync(candidate, "utf8")).name === name
    )
      return realpathSync(dirname(candidate));
  }
  const require = createRequire(resolve(from, "package.json"));
  let current = dirname(require.resolve(name));
  while (current !== dirname(current)) {
    const file = resolve(current, "package.json");
    if (
      existsSync(file) &&
      JSON.parse(readFileSync(file, "utf8")).name === name
    )
      return current;
    current = dirname(current);
  }
  throw new Error(`Cannot find installed media package: ${name}`);
}

export function inkFontInventory() {
  const directory = resolve(root, "frontend/public/fonts/clip");
  const metadata = JSON.parse(
    readFileSync(
      resolve(
        root,
        "frontend/src/entities/clip-design/config/clip-ink-fonts.json",
      ),
      "utf8",
    ),
  );
  const identity = JSON.parse(
    readFileSync(
      resolve(
        root,
        "frontend/src/entities/clip-design/config/clip-ink-identity.json",
      ),
      "utf8",
    ),
  );
  const resources = metadata.resources.map((resource) => {
    const bytes = readFileSync(resolve(directory, resource.file));
    if (bytes.byteLength !== resource.bytes || sha(bytes) !== resource.sha256)
      throw new Error(`Changed ink font: ${resource.file}`);
    return {
      ...resource,
      license: "OFL-1.1",
      notice: `frontend/public/fonts/clip/${resource.license}`,
    };
  });
  return {
    generator: metadata.generator,
    sourceVersion: metadata.wantedSourceVersion,
    manifestSHA256: identity.fontsManifestSHA256,
    assetVersion: identity.assetVersion,
    sources: Object.fromEntries(
      Object.entries(metadata.sources).map(([id, value]) => [
        id,
        { file: value.file, sha256: value.sha256 },
      ]),
    ),
    resources,
    modificationNotice: "frontend/public/fonts/clip/INK-NOTICE.txt",
    generatorSource: "scripts/build-clip-ink-fonts.py",
  };
}

// Cargo.lock is a package list, not proof of target/feature selection. Callers
// must supply runtime roots/host exclusions from the actual build manifests.
export function parseCargoLock(text) {
  const packages = [];
  for (const block of text.split(/^\[\[package\]\]\s*$/m).slice(1)) {
    const scalar = (key) => {
      const value = block.match(
        new RegExp(`^${key}\\s*=\\s*("(?:[^"\\\\]|\\\\.)*")`, "m"),
      );
      return value ? JSON.parse(value[1]) : null;
    };
    const name = scalar("name"),
      version = scalar("version");
    if (!name || !version) throw Error("Invalid Cargo.lock package identity");
    const dependency = block.match(/^dependencies\s*=\s*\[([\s\S]*?)\]/m);
    const dependencies = dependency
      ? [...dependency[1].matchAll(/"(?:[^"\\]|\\.)*"/g)].map((value) =>
          JSON.parse(value[0]),
        )
      : [];
    packages.push({
      name,
      version,
      source: scalar("source"),
      checksum: scalar("checksum"),
      dependencies,
    });
  }
  return packages;
}

export function deriveRustWasmGraph(
  lock,
  { roots, excluded = [], metadata = {} },
) {
  if (!roots?.length) throw Error("Explicit wasm runtime roots required");
  const packages = parseCargoLock(lock),
    graph = new Map(),
    unresolved = [];
  const excludedNames = new Set(excluded.map((entry) => entry.name));
  const visit = (dependency) => {
    const [name, version, ...origin] = dependency.split(" ");
    if (excludedNames.has(name)) return;
    const source = origin.join(" ").replace(/^\(|\)$/g, "");
    const matches = packages.filter(
      (entry) =>
        entry.name === name &&
        (!version || entry.version === version) &&
        (!source || entry.source === source),
    );
    if (matches.length !== 1) {
      unresolved.push({
        component: dependency,
        reason: "Lock dependency missing or ambiguous",
      });
      return;
    }
    const entry = matches[0],
      key = `${entry.name}@${entry.version}`;
    if (graph.has(key)) return;
    graph.set(key, {
      ...entry,
      declaredLicense: metadata[key]?.declaredLicense ?? null,
      sourceAccess: metadata[key]?.sourceAccess ?? null,
      notices: metadata[key]?.notices ?? [],
    });
    entry.dependencies.forEach(visit);
  };
  roots.forEach(visit);
  return {
    rustGraph: [...graph.values()].sort((a, b) =>
      `${a.name}@${a.version}`.localeCompare(`${b.name}@${b.version}`),
    ),
    unresolved,
    exclusions: excluded,
  };
}

// Rust panic locations expose only a lower bound. Do not turn these strings
// into an asserted complete dependency graph, or deduplicate distinct versions.
export function embeddedRustCrates(bytes) {
  const text = Buffer.from(bytes).toString("latin1"),
    crates = new Map();
  for (const [, name, version] of text.matchAll(
    /([A-Za-z0-9_-]+)-(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9._-]+)?)\/src\//g,
  ))
    crates.set(`${name}@${version}`, { name, version });
  return [...crates.values()].sort((a, b) =>
    `${a.name}@${a.version}`.localeCompare(`${b.name}@${b.version}`),
  );
}

export function auditRustEvidence({
  rustGraph,
  embeddedCrates,
  graphComplete,
  unresolved = [],
}) {
  const issues = [...unresolved];
  if (!graphComplete)
    issues.push({
      component: "Rust dependency graph",
      reason: "Complete original wasm runtime/feature graph is not verified",
    });
  for (const embedded of embeddedCrates) {
    const match = rustGraph.find(
      (entry) =>
        entry.name === embedded.name && entry.version === embedded.version,
    );
    if (!match)
      issues.push({
        component: `${embedded.name}@${embedded.version}`,
        reason: rustGraph.some((entry) => entry.name === embedded.name)
          ? "Embedded version does not match the derived graph"
          : "Embedded crate absent from verified graph; exact-version source, declared license and notice remain unresolved",
      });
  }
  for (const entry of rustGraph) {
    if (
      !entry.declaredLicense ||
      !entry.sourceAccess?.versionPinned ||
      !entry.sourceAccess?.url ||
      !entry.notices?.length ||
      entry.notices.some(
        (notice) =>
          !notice.file ||
          !/^[a-f0-9]{64}$/.test(notice.sha256 ?? "") ||
          !notice.provenance,
      )
    )
      issues.push({
        component: `${entry.name}@${entry.version}`,
        reason:
          "Exact-version source, declared license or original notice is missing",
      });
  }
  return {
    unresolved: issues,
    sourceAccessVerified:
      graphComplete === true && rustGraph.length > 0 && issues.length === 0,
  };
}

function resvgRustInventory(directory, version) {
  const publicRoot = resolve(root, "frontend/public/licenses/browser-media");
  const evidence = JSON.parse(
    readFileSync(resolve(publicRoot, "resvg-rust-evidence.json"), "utf8"),
  );
  const wasm = readFileSync(resolve(directory, "index_bg.wasm"));
  if (
    evidence.npmVersion !== version ||
    evidence.binary.sha256 !== sha(wasm) ||
    evidence.binary.bytes !== wasm.length
  )
    throw Error("Retained Rust evidence does not match installed resvg WASM");
  if (sha(evidence.cargoManifest.content) !== evidence.cargoManifest.sha256)
    throw Error("Retained original Cargo.toml content/hash changed");
  let derived = { rustGraph: [], unresolved: [] };
  if (evidence.cargoLock.present) {
    if (
      typeof evidence.cargoLock.content !== "string" ||
      sha(evidence.cargoLock.content) !== evidence.cargoLock.sha256
    )
      throw Error("Original Cargo.lock content/hash is missing or changed");
    derived = deriveRustWasmGraph(evidence.cargoLock.content, {
      roots: evidence.runtimeRoots,
      excluded: evidence.exclusions,
      metadata: evidence.crateMetadata,
    });
  }
  const rustGraph = derived.rustGraph.map((entry) => ({
    ...entry,
    notices: (entry.notices ?? []).filter((notice) => {
      if (!/^[A-Za-z0-9_.+-]+-LICENSE$/.test(notice.file ?? "")) return false;
      const file = resolve(publicRoot, notice.file);
      return existsSync(file) && sha(readFileSync(file)) === notice.sha256;
    }),
  }));
  const embeddedCrates = embeddedRustCrates(wasm);
  const audit = auditRustEvidence({
    rustGraph,
    embeddedCrates,
    graphComplete:
      evidence.graphComplete === true &&
      evidence.cargoLock.present === true &&
      evidence.targetSelectionVerified === true,
    unresolved: [...evidence.unresolved, ...derived.unresolved],
  });
  return {
    rustGraph,
    embeddedCrates,
    rustBuildEvidence: evidence,
    unresolved: audit.unresolved,
    sourceAccessVerified: audit.sourceAccessVerified,
  };
}

async function writeRustDistribution(inventory) {
  const directory = resolve(root, "frontend/public/licenses/browser-media");
  const file = resolve(directory, "source-access.json");
  const manifest = JSON.parse(readFileSync(file, "utf8"));
  const source = inventory.packages.find(
    (entry) => entry.name === "@resvg/resvg-wasm",
  );
  const target = manifest.packages.find(
    (entry) => entry.name === source.name && entry.version === source.version,
  );
  if (!target) throw Error("Missing matching resvg distribution entry");
  for (const key of [
    "rustGraph",
    "embeddedCrates",
    "rustBuildEvidence",
    "unresolved",
    "sourceAccessVerified",
  ])
    target[key] = source[key];
  manifest.unresolved = inventory.unresolved;
  manifest.sourceAccessVerified = false;
  manifest.binaryDependencyGraphVerified = source.sourceAccessVerified;
  manifest.distributionApproved = false;
  const preferred = manifest.coveredSources.find(
    (entry) => entry.name === source.name,
  );
  if (preferred) {
    preferred.completeOriginalBinaryGraphVerified =
      manifest.binaryDependencyGraphVerified;
    preferred.limit =
      "The original build Cargo.lock is absent; embedded crate paths are a lower bound, not a resolved target graph.";
  }
  writeFileSync(file, JSON.stringify(manifest, null, 2) + "\n");
  const textMarker = "\n\nRust WASM provenance inventory (generated)\n";
  const textFile = resolve(directory, "THIRD-PARTY.txt");
  const text = readFileSync(textFile, "utf8").split(textMarker)[0];
  const issues = source.unresolved.map(
    (entry) => `${entry.component}: ${entry.reason}`,
  );
  writeFileSync(
    textFile,
    text +
      textMarker +
      "This inventory does not replace missing original notices. See source-access.json for exact binary/source binding.\n" +
      issues.join("\n") +
      "\n",
  );
  const htmlFile = resolve(directory, "index.html");
  const html = readFileSync(htmlFile, "utf8").replace(
    /\s*<section id="rust-provenance">[\s\S]*?<\/section>/,
    "",
  );
  const escape = (value) =>
    value
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;");
  const section = `\n<section id="rust-provenance"><h2>Original resvg WASM Rust provenance</h2><p>${source.embeddedCrates.length} embedded crate/version strings are a lower bound. The original build lockfile and complete Rust graph are unavailable; exact source/license/notice gaps remain open in <a href="source-access.json">the manifest</a>.</p><ul>${issues.map((issue) => `<li>${escape(issue)}</li>`).join("\n")}</ul></section>\n`;
  const frontendRequire = createRequire(resolve(root, "frontend/package.json"));
  const prettier = frontendRequire("prettier");
  const options = await prettier.resolveConfig(htmlFile);
  const plugins = (options?.plugins ?? []).map((plugin) =>
    typeof plugin === "string" ? frontendRequire.resolve(plugin) : plugin,
  );
  writeFileSync(
    htmlFile,
    await prettier.format(html.replace("</main>", section + "</main>"), {
      ...options,
      plugins,
      filepath: htmlFile,
    }),
  );
}

export function installedMediaInventory() {
  const queue = topLevel.map((name) => ({
    name,
    from: resolve(root, "frontend"),
  }));
  const packages = new Map();
  const unresolved = [];
  while (queue.length) {
    const { name, from } = queue.shift();
    let directory;
    try {
      directory = packageDirectory(name, from);
    } catch {
      unresolved.push(name);
      continue;
    }
    const bytes = readFileSync(resolve(directory, "package.json"));
    const metadata = JSON.parse(bytes);
    const identity = `${metadata.name}@${metadata.version}`;
    if (packages.has(identity)) continue;
    const notices = [
      "LICENSE",
      "LICENSE.md",
      "LICENSE.txt",
      "LICENSE-MIT",
      "NOTICE",
      "NOTICE.txt",
    ]
      .filter((file) => existsSync(resolve(directory, file)))
      .map((file) => ({
        file,
        sha256: sha(readFileSync(resolve(directory, file))),
      }));
    const rust =
      metadata.name === "@resvg/resvg-wasm"
        ? resvgRustInventory(directory, metadata.version)
        : null;
    if (rust)
      unresolved.push(
        ...rust.unresolved.map(
          (issue) => `${metadata.name}: ${issue.component}: ${issue.reason}`,
        ),
      );
    packages.set(identity, {
      name: metadata.name,
      version: metadata.version,
      declaredLicense: metadata.license ?? null,
      repository: repositoryURL(metadata.repository),
      packageMetadataSHA256: sha(bytes),
      packagedNotices: notices,
      obligation: licenseObligation(metadata.license),
      topLevel: topLevel.includes(metadata.name),
      coveredSourceEvidence: null,
      ...(rust ?? {}),
    });
    for (const dependency of Object.keys(metadata.dependencies ?? {}))
      queue.push({ name: dependency, from: directory });
  }
  const fonts = [
    "LICENSE",
    "LICENSE-Paperlogy",
    "LICENSE-Jua",
    "LICENSE-NanumMyeongjo",
  ].map((file) => ({
    file: `frontend/public/fonts/clip/${file}`,
    license: "OFL-1.1",
    sha256: sha(
      readFileSync(resolve(root, "frontend/public/fonts/clip", file)),
    ),
  }));
  return {
    version: 1,
    lockfileSHA256: sha(readFileSync(resolve(root, "pnpm-lock.yaml"))),
    packages: [...packages.values()].sort((a, b) =>
      a.name.localeCompare(b.name),
    ),
    unresolved: [...new Set(unresolved)].sort(),
    fonts,
    inkFonts: inkFontInventory(),
    sourceAccessVerified: false,
    binaryDependencyGraphVerified:
      [...packages.values()].find((value) => value.name === "@resvg/resvg-wasm")
        ?.sourceAccessVerified === true,
    distributionApproved: false,
    limits: [
      "Dependency metadata is an inventory, not a substitute for actual bundled-license review.",
      "resvg-wasm embeds Rust dependencies whose versioned source/licenses require build-provenance review.",
      "The production FFmpeg libx264 build is GPL-enabled; this browser inventory does not redistribute it.",
      "Remotion is an optional comparison under its own license; it is not installed in the production bundle.",
    ],
  };
}

async function verifyCoveredSources(inventory) {
  const evidence = {
    mediabunny: (version) =>
      `https://raw.githubusercontent.com/Vanilagy/mediabunny/v${version}/LICENSE`,
    "@resvg/resvg-wasm": (version) =>
      `https://raw.githubusercontent.com/thx/resvg-js/v${version}/LICENSE`,
  };
  for (const value of inventory.packages) {
    if (!value.topLevel || value.declaredLicense !== "MPL-2.0") continue;
    let url = evidence[value.name]?.(value.version);
    if (value.name === "@soundtouchjs/core") {
      const metadataResponse = await fetch(
        `https://registry.npmjs.org/${encodeURIComponent(value.name)}/${value.version}`,
        { signal: AbortSignal.timeout(20_000) },
      );
      if (!metadataResponse.ok)
        throw new Error("SoundTouch source revision unavailable");
      const metadata = await metadataResponse.json();
      if (metadata.name !== value.name || metadata.version !== value.version)
        throw new Error("SoundTouch source revision is not version-bound");
      let revision = metadata.gitHead;
      if (!/^[a-f0-9]{40}$/.test(revision ?? "")) {
        const reference = await fetch(
          `https://api.github.com/repos/cutterbl/SoundTouchJS/git/ref/tags/v${value.version}`,
          { signal: AbortSignal.timeout(20_000) },
        ).then((response) => {
          if (!response.ok)
            throw new Error("SoundTouch version tag unavailable");
          return response.json();
        });
        revision = reference.object?.sha;
        if (reference.object?.type === "tag") {
          const tag = await fetch(
            `https://api.github.com/repos/cutterbl/SoundTouchJS/git/tags/${revision}`,
            { signal: AbortSignal.timeout(20_000) },
          ).then((response) => response.json());
          revision = tag.object?.type === "commit" ? tag.object.sha : null;
        }
      }
      if (!/^[a-f0-9]{40}$/.test(revision ?? ""))
        throw new Error("SoundTouch immutable source revision unavailable");
      const sourceMetadata = await fetch(
        `https://raw.githubusercontent.com/cutterbl/SoundTouchJS/${revision}/packages/core/package.json`,
        { signal: AbortSignal.timeout(20_000) },
      ).then((response) => response.json());
      if (
        sourceMetadata.name !== value.name ||
        sourceMetadata.version !== value.version
      )
        throw new Error(
          "SoundTouch source does not match the installed version",
        );
      url = `https://raw.githubusercontent.com/cutterbl/SoundTouchJS/${revision}/packages/core/LICENSE`;
      value.sourceRevision = revision;
    }
    if (!url) continue;
    const response = await fetch(url, { signal: AbortSignal.timeout(20_000) });
    if (!response.ok)
      throw new Error(`Covered source evidence unavailable: ${value.name}`);
    const body = await response.text();
    if (!body.includes("Mozilla Public License"))
      throw new Error(`Unexpected license evidence: ${value.name}`);
    value.coveredSourceEvidence = {
      url,
      sha256: sha(body),
      versionPinned: true,
    };
    if (value.name === "@resvg/resvg-wasm") {
      const cargoURL = `https://raw.githubusercontent.com/thx/resvg-js/v${value.version}/Cargo.toml`;
      const cargoResponse = await fetch(cargoURL, {
        signal: AbortSignal.timeout(20_000),
      });
      if (!cargoResponse.ok)
        throw new Error("resvg WASM build source unavailable");
      const cargo = await cargoResponse.text();
      const nativeBuild = readFileSync(
        resolve(root, "backend/build/render-tools.sh"),
        "utf8",
      );
      value.wasmSourceDeclaration = {
        url: cargoURL,
        sha256: sha(cargo),
        declaredResvgVersion:
          cargo.match(/resvg\s*=\s*\{\s*version\s*=\s*"([^"]+)"/)?.[1] ?? null,
        patchedResvgRevision:
          cargo.match(
            /resvg\s*=\s*\{\s*git\s*=\s*"[^"]+"\s*,\s*rev\s*=\s*"([^"]+)"/,
          )?.[1] ?? null,
        nativeReferenceVersion:
          nativeBuild.match(/RESVG_VERSION=([^\s]+)/)?.[1] ?? null,
        binaryDependencyGraphVerified: false,
        limit:
          "Published WASM binary provenance and patched Rust dependency licenses still require release review; native/WASM renderer versions are not assumed equivalent.",
      };
    }
  }
  inventory.upstreamLicenseEvidenceVerified = inventory.packages
    .filter((value) => value.topLevel && value.declaredLicense === "MPL-2.0")
    .every((value) => value.coveredSourceEvidence?.versionPinned);
  // A fetched license is not recipient access to complete covered sources,
  // including the original executable's embedded Rust graph and modifications.
  inventory.sourceAccessVerified = false;
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === resolve(process.argv[1])
) {
  const inventory = installedMediaInventory();
  if (process.argv.includes("--verify-upstream"))
    await verifyCoveredSources(inventory);
  if (process.argv.includes("--write-distribution"))
    await writeRustDistribution(inventory);
  const index = process.argv.indexOf("--output");
  const output = JSON.stringify(inventory, null, 2) + "\n";
  if (index !== -1) writeFileSync(resolve(process.argv[index + 1]), output);
  else process.stdout.write(output);
}
