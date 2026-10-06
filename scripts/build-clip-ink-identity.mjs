import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const config = resolve(root, "frontend/src/entities/clip-design/config");
const canonical = (value) =>
  JSON.stringify(value, (_key, item) =>
    item && typeof item === "object" && !Array.isArray(item)
      ? Object.fromEntries(
          Object.entries(item).sort(([a], [b]) => a.localeCompare(b)),
        )
      : item,
  );
const hash = (value) =>
  createHash("sha256").update(canonical(value)).digest("hex");
const fonts = JSON.parse(
  readFileSync(resolve(config, "clip-ink-fonts.json"), "utf8"),
);
const catalog = JSON.parse(
  readFileSync(resolve(config, "clip-caption-ink.json"), "utf8"),
);
const material = {
  manifestVersion: 1,
  inkVersion: "clip-ink-v2-resvg-2.6.2",
  transformInkScale: 2,
  transformInkStyles: ["word-pop", "pop", "stack", "sticker", "bubble"],
  fontsManifestSHA256: hash(fonts),
  catalogSHA256: hash(catalog),
  wasmSHA256:
    "22bf6e9f9a100d972da0411a69c5ba504367fc1fa87b3b64e3f35e53926d2d70",
  fontAssets: fonts.resources.map((font) => ({
    id: `${font.face}:${font.weight}`,
    sha256: font.sha256,
  })),
};
const identity = {
  ...material,
  assetVersion: `clip-design-assets-v1-ink-${hash(material)}`,
};
writeFileSync(
  resolve(config, "clip-ink-identity.json"),
  JSON.stringify(identity, null, 2) + "\n",
);
for (const [file, expression] of [
  [resolve(config, "browser-composition.ts"), /assets:\s*'[^']+'/u],
  [
    resolve(root, "backend/internal/clip/browser_render.go"),
    /const BrowserAssetVersion = "[^"]+"/u,
  ],
]) {
  const source = readFileSync(file, "utf8");
  if (!expression.test(source))
    throw new Error(`Missing resource version in ${file}`);
  const replacement = file.endsWith(".ts")
    ? `assets: '${identity.assetVersion}'`
    : `const BrowserAssetVersion = "${identity.assetVersion}"`;
  writeFileSync(file, source.replace(expression, replacement));
}
console.log(identity.assetVersion);
