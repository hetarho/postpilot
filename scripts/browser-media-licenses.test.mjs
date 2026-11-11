import assert from "node:assert/strict";
import test from "node:test";
import {
  installedMediaInventory,
  deriveRustWasmGraph,
  embeddedRustCrates,
  auditRustEvidence,
  licenseObligation,
  repositoryURL,
} from "./browser-media-licenses.mjs";

test("inventory follows installed runtime dependencies and retains unresolved distribution gates", () => {
  const inventory = installedMediaInventory();
  assert(inventory.unresolved.length > 0);
  assert(
    inventory.unresolved.every((issue) =>
      issue.startsWith("@resvg/resvg-wasm:"),
    ),
  );
  const rust = inventory.packages.find(
    (value) => value.name === "@resvg/resvg-wasm",
  );
  assert.equal(rust.rustBuildEvidence.cargoLock.present, false);
  assert.equal(rust.rustGraph.length, 0);
  assert.equal(rust.embeddedCrates.length, 39);
  assert(rust.unresolved.some((issue) => issue.reason.includes("Cargo.lock")));
  for (const crate of rust.embeddedCrates)
    assert(
      rust.unresolved.some(
        (issue) => issue.component === `${crate.name}@${crate.version}`,
      ),
    );
  assert.equal(
    inventory.packages.find((value) => value.name === "mediabunny").version,
    "1.58.0",
  );
  assert.equal(
    inventory.packages.find((value) => value.name === "@resvg/resvg-wasm")
      .declaredLicense,
    "MPL-2.0",
  );
  assert(
    inventory.packages.some(
      (value) => value.name === "@soundtouchjs/interpolation-strategy-lanczos",
    ),
  );
  assert.equal(inventory.fonts.length, 4);
  assert.equal(inventory.inkFonts.resources.length, 8);
  assert.equal(
    inventory.inkFonts.resources.filter((font) => font.face === "wantedsans")
      .length,
    4,
  );
  assert.match(inventory.inkFonts.manifestSHA256, /^[a-f0-9]{64}$/);
  assert.match(inventory.lockfileSHA256, /^[a-f0-9]{64}$/);
  assert.equal(inventory.distributionApproved, false);
  assert.equal(inventory.sourceAccessVerified, false);
});

test("source and obligations distinguish MPL, permissive licenses and uncertain expressions", () => {
  assert.equal(
    repositoryURL("git@github.com:yisibl/resvg-js.git"),
    "https://github.com/yisibl/resvg-js",
  );
  assert.equal(
    repositoryURL({ url: "git+https://github.com/pixijs/pixijs.git" }),
    "https://github.com/pixijs/pixijs",
  );
  assert.match(licenseObligation("MPL-2.0"), /covered source/);
  assert.match(licenseObligation("MIT"), /copyright/);
  assert.match(licenseObligation("MIT OR CUSTOM"), /Review/);
});

const lock = `version = 3
[[package]]
name = "renderer"
version = "1.0.0"
dependencies = ["shared", "png 0.17.5", "native-only", "host-build"]
[[package]]
name = "shared"
version = "2.0.0"
source = "registry+https://github.com/rust-lang/crates.io-index"
dependencies = ["png 0.17.5"]
[[package]]
name = "png"
version = "0.17.5"
source = "registry+https://github.com/rust-lang/crates.io-index"
[[package]]
name = "png"
version = "0.18.0"
[[package]]
name = "native-only"
version = "9.0.0"
[[package]]
name = "host-build"
version = "8.0.0"
[[package]]
name = "dev-only"
version = "7.0.0"
`;
test("explicit wasm roots exclude native, build and unreachable dev packages", () => {
  const result = deriveRustWasmGraph(lock, {
    roots: ["renderer"],
    excluded: [
      { name: "native-only", reason: "non-wasm dependency" },
      { name: "host-build", reason: "host build dependency" },
    ],
  });
  assert.deepEqual(
    result.rustGraph.map((p) => `${p.name}@${p.version}`),
    ["png@0.17.5", "renderer@1.0.0", "shared@2.0.0"],
  );
  assert.equal(
    result.rustGraph[0].source,
    "registry+https://github.com/rust-lang/crates.io-index",
  );
  assert.equal(result.rustGraph[0].declaredLicense, null);
  assert.equal(result.unresolved.length, 0);
  const ambiguous = deriveRustWasmGraph(lock, { roots: ["png"] });
  assert.match(ambiguous.unresolved[0].reason, /ambiguous/);
});
test("binary panic paths retain multiple versions and ignore unrelated bytes", () => {
  const bytes = Buffer.concat([
    Buffer.from([0, 255, 1]),
    Buffer.from(
      "/registry/src/index/png-0.17.5/src/lib.rs\0/s/svgtypes-0.11.0/src/lib.rs\0/s/svgtypes-0.14.0/src/lib.rs\0/png-0.17.5/src/lib.rs\0/not-a-version-hello/src/",
    ),
  ]);
  assert.deepEqual(embeddedRustCrates(bytes), [
    { name: "png", version: "0.17.5" },
    { name: "svgtypes", version: "0.11.0" },
    { name: "svgtypes", version: "0.14.0" },
  ]);
});
test("embedded version mismatch and absent crate stay unresolved", () => {
  const result = auditRustEvidence({
    rustGraph: [{ name: "png", version: "0.18.0" }],
    embeddedCrates: [
      { name: "png", version: "0.17.5" },
      { name: "unknown", version: "1.0.0" },
    ],
    graphComplete: true,
  });
  assert.match(
    result.unresolved.find((x) => x.component === "png@0.17.5").reason,
    /version does not match/,
  );
  assert.match(
    result.unresolved.find((x) => x.component === "unknown@1.0.0").reason,
    /absent/,
  );
  assert.equal(result.sourceAccessVerified, false);
});
test("source access stays false for missing original notice or incomplete graph", () => {
  const crate = {
    name: "png",
    version: "0.17.5",
    declaredLicense: "MIT",
    sourceAccess: {
      versionPinned: true,
      url: "https://example.invalid/commit",
    },
    notices: [],
  };
  const result = auditRustEvidence({
    rustGraph: [crate],
    embeddedCrates: [{ name: "png", version: "0.17.5" }],
    graphComplete: true,
  });
  assert.equal(result.sourceAccessVerified, false);
  assert.match(result.unresolved[0].reason, /notice is missing/);
  crate.notices = [
    {
      file: "png0.17.5-LICENSE",
      sha256: "a".repeat(64),
      provenance: { url: "https://example.invalid/commit/LICENSE" },
    },
  ];
  assert.equal(
    auditRustEvidence({
      rustGraph: [crate],
      embeddedCrates: [{ name: "png", version: "0.17.5" }],
      graphComplete: true,
    }).sourceAccessVerified,
    true,
  );
  assert.equal(
    auditRustEvidence({
      rustGraph: [crate],
      embeddedCrates: [],
      graphComplete: false,
    }).sourceAccessVerified,
    false,
  );
  assert.equal(
    auditRustEvidence({
      rustGraph: [],
      embeddedCrates: [],
      graphComplete: true,
    }).sourceAccessVerified,
    false,
  );
});
