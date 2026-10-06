import {
  createReadStream,
  readFileSync,
  statSync,
  mkdirSync,
  writeFileSync,
} from "node:fs";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { chromium } from "playwright";
import { parseByteRange } from "./browser-media-report.mjs";

const root = resolve(import.meta.dirname, ".."),
  args = process.argv.slice(2);
const option = (name, fallback) =>
  args.includes(name) ? args[args.indexOf(name) + 1] : fallback;
const output = resolve(option("--output", "tmp/browser-export-check"));
const files = new Map([
  [
    "silent",
    option(
      "--video",
      "/private/tmp/postpilot-browser-media-prep-video/original-cfr60.mp4",
    ),
  ],
  [
    "source",
    option(
      "--audio-video",
      "/private/tmp/postpilot-browser-media-prep-audio/delayed-audio.mp4",
    ),
  ],
  [
    "speech",
    option(
      "--speech",
      "/private/tmp/postpilot-browser-media-prep-audio/synthetic-speech.wav",
    ),
  ],
]);
const require = createRequire(resolve(root, "frontend/package.json"));
const { createServer } = await import(
  pathToFileURL(require.resolve("vite")).href
);
const server = await createServer({
  configFile: false,
  root: resolve(root, "frontend"),
  cacheDir: resolve(output, "vite-cache"),
  resolve: { alias: { "@": resolve(root, "frontend/src") } },
  server: { host: "127.0.0.1", port: 0, hmr: false, watch: null },
  plugins: [
    {
      name: "export-fixture",
      configureServer(vite) {
        vite.middlewares.use((request, response, next) => {
          response.setHeader("Cross-Origin-Opener-Policy", "same-origin");
          response.setHeader("Cross-Origin-Embedder-Policy", "require-corp");
          if (request.url === "/probe") {
            response.setHeader("Content-Type", "text/html");
            response.end(
              '<script type=module src="/src/test/browser-media/export-entry.ts"></script>',
            );
            return;
          }
          const path = files.get(request.url?.slice("/file/".length));
          if (!request.url?.startsWith("/file/") || !path) return next();
          const size = statSync(path).size,
            range = parseByteRange(request.headers.range, size);
          response.writeHead(range ? 206 : 200, {
            "Content-Type": path.endsWith(".wav") ? "audio/wav" : "video/mp4",
            "Content-Length": range ? range.end - range.start + 1 : size,
            ...(range
              ? { "Content-Range": `bytes ${range.start}-${range.end}/${size}` }
              : {}),
          });
          const stream = createReadStream(path, range ?? {});
          response.on("close", () => stream.destroy());
          stream.pipe(response);
        });
      },
    },
  ],
});
await server.listen();
const origin = server.resolvedUrls.local[0].replace(/\/$/, "");
const executablePath = option(
  "--browser",
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
);
const browser = await chromium.launch({ executablePath });
try {
  const page = await browser.newPage();
  await page.goto(origin + "/probe");
  await page.waitForFunction(() => !!window.exportFixture);
  const storage = await page.evaluate(() => window.exportFixture.storage());
  if (
    storage.kind !== "opfs" ||
    JSON.stringify(storage.bytes) !==
      JSON.stringify([1, 2, 3, 13, 14, 15, 0, 0, 8, 9, 10, 11]) ||
    storage.liveNames.length !== 1 ||
    storage.remaining.length ||
    storage.afterCancel.length ||
    storage.overflow !== "MEDIA_OUTPUT_SIZE_LIMIT"
  )
    throw Error(JSON.stringify(storage));
  await page.evaluate(() => window.exportFixture.leaveAbandoned());
  await page.goto("about:blank");
  await page.goto(origin + "/probe");
  await page.waitForFunction(() => !!window.exportFixture);
  const recovery = await page.evaluate(() => window.exportFixture.recover());
  if (recovery.before.length !== 1 || recovery.after.length)
    throw Error(JSON.stringify(recovery));
  const cases = [];
  for (const [mode, ratio, memory] of [
    ["silent", "vertical"],
    ["silent", "horizontal"],
    ["silent", "square"],
    ["source", "square"],
    ["narration", "vertical"],
    ["mixed", "square"],
    ["silent", "square", true],
  ]) {
    const source = mode === "source" || mode === "mixed" ? "source" : "silent";
    const fingerprint = createHash("sha256")
      .update(readFileSync(files.get(source)))
      .digest("hex");
    const result = await page.evaluate(
      (request) => window.exportFixture.render(request),
      {
        mode,
        ratio,
        memory,
        url: origin + "/file/" + source,
        fingerprint,
        speechUrl: origin + "/file/speech",
        slowMs: mode === "silent" && ratio === "vertical" ? 5 : 0,
      },
    );
    if (
      result.error ||
      !result.verdict.passed ||
      result.frameCount !== 450 ||
      result.output.videoFrames !== 450 ||
      result.retainedVideoPackets !== 0 ||
      result.kind !== (memory ? "blob" : "opfs") ||
      result.packetResources.peakPackets > 8 ||
      result.packetResources.peakBytes > 4 * 1024 * 1024 ||
      result.sourceResources.presentation.liveFrames !== 0 ||
      result.seeks.length !== 3
    )
      throw Error(JSON.stringify(result));
    if ((mode === "narration" || mode === "mixed") && !result.speechFingerprint)
      throw Error("Missing exact speech provenance");
    cases.push(result);
    console.log(
      JSON.stringify({
        mode,
        ratio,
        kind: result.kind,
        decodedAudio: result.decodedAudio,
        bytes: result.fileBytes,
        packets: result.packetResources,
        passed: true,
      }),
    );
  }
  const fingerprint = createHash("sha256")
    .update(readFileSync(files.get("silent")))
    .digest("hex");
  const cancelled = await page.evaluate(
    (request) => window.exportFixture.render(request),
    {
      mode: "silent",
      ratio: "square",
      url: origin + "/file/silent",
      fingerprint,
      speechUrl: origin + "/file/speech",
      cancelAt: 1,
    },
  );
  if (cancelled.name !== "AbortError") throw Error(JSON.stringify(cancelled));
  const afterCancel = await page.evaluate(() => window.exportFixture.recover());
  if (afterCancel.before.length || afterCancel.after.length)
    throw Error("Cancelled output persisted");
  const report = {
    version: 1,
    qualification: false,
    scope:
      "Actual Chrome production Worker, codecs, standard seekable MP4, synthetic requested audio and origin-output ownership. No hardware, real-voice, native full-composition or release qualification.",
    browser: await browser.version(),
    executablePath,
    node: process.version,
    fixtures: Object.fromEntries(
      [...files].map(([id, path]) => [
        id,
        {
          bytes: statSync(path).size,
          sha256: createHash("sha256").update(readFileSync(path)).digest("hex"),
        },
      ]),
    ),
    storage,
    recovery,
    cases,
    cancelled,
    afterCancel,
  };
  mkdirSync(output, { recursive: true });
  writeFileSync(
    resolve(output, "report.json"),
    JSON.stringify(report, null, 2) + "\n",
  );
} finally {
  await browser.close();
  await server.close();
}
