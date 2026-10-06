import { copyFileSync, existsSync, readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const isolated = resolve(
  process.argv[2] ?? "/tmp/postpilot-remotion-benchmark",
);
if (!existsSync(resolve(isolated, "package.json")))
  throw new Error(
    "Install the comparison SDKs in the explicitly isolated diagnostic workspace first",
  );
copyFileSync(
  resolve(root, "scripts/experiments/remotion-browser.jsx"),
  resolve(isolated, "remotion-browser.jsx"),
);
const require = createRequire(resolve(isolated, "package.json"));
const { build } = await import(pathToFileURL(require.resolve("vite")).href);
await build({
  configFile: false,
  root: isolated,
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
  build: {
    outDir: resolve(root, "tmp/browser-media/remotion-experiment"),
    lib: {
      entry: resolve(isolated, "remotion-browser.jsx"),
      formats: ["es"],
      fileName: () => "adapter.mjs",
    },
  },
});
const packages = [
  "remotion",
  "@remotion/web-renderer",
  "@remotion/media",
  "@remotion/licensing",
  "mediabunny",
  "react",
  "react-dom",
  "@mediabunny/aac-encoder",
  "@mediabunny/mp3-encoder",
  "@mediabunny/flac-encoder",
].map((name) => {
  const path = resolve(isolated, "node_modules", name, "package.json");
  const metadata = JSON.parse(readFileSync(path, "utf8"));
  const license = ["LICENSE.md", "LICENSE", "LICENSE.txt"]
    .map((file) => resolve(dirname(path), file))
    .find(existsSync);
  return {
    name,
    version: metadata.version,
    declaredLicense: metadata.license ?? "unverified",
    licenseSHA256: license
      ? createHash("sha256").update(readFileSync(license)).digest("hex")
      : null,
  };
});
writeFileSync(
  resolve(root, "tmp/browser-media/remotion-experiment/metadata.json"),
  JSON.stringify(
    {
      packages,
      isolatedLockfileSHA256: createHash("sha256")
        .update(readFileSync(resolve(isolated, "package-lock.json")))
        .digest("hex"),
      conditions:
        "Remotion custom license; development evaluation only, free-license declaration, isProduction false. Check current eligibility and telemetry before production use.",
      productionDependency: false,
      optionalCodecBinaries: [
        {
          package: "@mediabunny/aac-encoder",
          implementation: "FFmpeg AAC WASM",
          wrapperLicense: "MPL-2.0",
          binaryVersion: null,
          binaryLicenseVerified: false,
        },
        {
          package: "@mediabunny/mp3-encoder",
          implementation: "LAME 3.100 WASM",
          wrapperLicense: "MPL-2.0",
          binaryLicenseDeclaration:
            "LGPL; exact build/license/source-access obligations remain unverified",
          binaryLicenseVerified: false,
        },
        {
          package: "@mediabunny/flac-encoder",
          implementation: "libFLAC WASM",
          wrapperLicense: "MPL-2.0",
          binaryVersion: null,
          binaryLicenseVerified: false,
        },
      ],
    },
    null,
    2,
  ) + "\n",
);
