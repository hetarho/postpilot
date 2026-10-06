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
  inventory.sourceAccessVerified = inventory.packages
    .filter((value) => value.topLevel && value.declaredLicense === "MPL-2.0")
    .every((value) => value.coveredSourceEvidence?.versionPinned);
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === resolve(process.argv[1])
) {
  const inventory = installedMediaInventory();
  if (process.argv.includes("--verify-upstream"))
    await verifyCoveredSources(inventory);
  const index = process.argv.indexOf("--output");
  const output = JSON.stringify(inventory, null, 2) + "\n";
  if (index !== -1) writeFileSync(resolve(process.argv[index + 1]), output);
  else process.stdout.write(output);
}
