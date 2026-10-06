import assert from "node:assert/strict";
import test from "node:test";
import {
  installedMediaInventory,
  licenseObligation,
  repositoryURL,
} from "./browser-media-licenses.mjs";

test("inventory follows installed runtime dependencies and retains unresolved distribution gates", () => {
  const inventory = installedMediaInventory();
  assert.equal(inventory.unresolved.length, 0);
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
