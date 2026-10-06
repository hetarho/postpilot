import {
  createReadStream,
  readFileSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { mkdirSync } from "node:fs";
import { pathToFileURL } from "node:url";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";

const root = resolve(import.meta.dirname, "..");
const args = process.argv.slice(2);
const option = (name, fallback) =>
  args.includes(name) ? args[args.indexOf(name) + 1] : fallback;
const output = resolve(option("--output", "tmp/browser-background-check"));
const fixtures = resolve(output, "fixtures");
const ffmpeg = option("--ffmpeg", "ffmpeg");
const ffprobe = option("--ffprobe", "ffprobe");
mkdirSync(fixtures, { recursive: true });
const definitions = [
  [
    "full709",
    "color=c=red:s=64x32:r=60:d=2,scale=in_color_matrix=bt601:out_color_matrix=bt709:in_range=tv:out_range=pc",
    true,
    "pc",
  ],
  [
    "hdr",
    "color=c=white:s=64x32:r=60:d=2,setparams=range=tv:color_primaries=bt709:color_trc=arib-std-b67:colorspace=bt709",
    true,
    "tv",
    "arib-std-b67",
  ],
  ["untagged", "color=c=red:s=64x32:r=60:d=2", false],
  [
    "tagged709",
    "color=c=red:s=64x32:r=60:d=2,scale=in_color_matrix=bt601:out_color_matrix=bt709",
    true,
  ],
  ...[203, 204, 205].map((v) => [
    "gray" + v,
    `color=c=0x${v.toString(16).repeat(3)}:s=64x32:r=60:d=2,scale=in_color_matrix=bt601:out_color_matrix=bt709`,
    true,
  ]),
  ["dark", "color=c=black:s=64x32:r=60:d=2", true],
  ["bright", "color=c=white:s=64x32:r=60:d=2", true],
  [
    "noisy",
    "color=c=black:s=64x32:r=60:d=2,drawbox=x=0:y=0:w=iw:h=ih:color=white:t=fill:enable='between(t,0.45,0.55)'",
    true,
  ],
];
const files = new Map();
for (const [
  id,
  graph,
  tagged,
  range = "tv",
  transfer = "bt709",
] of definitions) {
  const path = resolve(fixtures, id + ".mp4");
  const result = spawnSync(ffmpeg, [
    "-hide_banner",
    "-nostdin",
    "-v",
    "error",
    "-y",
    "-f",
    "lavfi",
    "-i",
    graph,
    "-an",
    "-c:v",
    "libx264",
    "-pix_fmt",
    "yuv420p",
    "-crf",
    "0",
    ...(tagged
      ? [
          "-colorspace",
          "bt709",
          "-color_primaries",
          "bt709",
          "-color_trc",
          transfer,
          "-color_range",
          range,
        ]
      : []),
    path,
  ]);
  if (result.status !== 0)
    throw new Error("Fixture generation failed: " + result.stderr.toString());
  files.set(id, path);
}
const require = createRequire(resolve(root, "frontend/package.json"));
const { createServer } = await import(
  pathToFileURL(require.resolve("vite")).href
);
const { chromium } = require("playwright");
const { parseByteRange } = await import(
  pathToFileURL(resolve(root, "scripts/browser-media-report.mjs")).href
);
const server = await createServer({
  configFile: false,
  root: resolve(root, "frontend"),
  cacheDir: resolve(output, "vite-cache"),
  resolve: { alias: { "@": resolve(root, "frontend/src") } },
  server: { host: "127.0.0.1", port: 0, hmr: false, watch: null },
  plugins: [
    {
      name: "t598-color-fixtures",
      configureServer(vite) {
        vite.middlewares.use((request, response, next) => {
          response.setHeader("Cross-Origin-Opener-Policy", "same-origin");
          response.setHeader("Cross-Origin-Embedder-Policy", "require-corp");
          if (request.url === "/probe") {
            response.setHeader("Content-Type", "text/html");
            response.end(
              '<script type=module src="/src/test/browser-media/video-range-entry.ts"></script><script type=module src="/src/test/browser-media/background-entry.ts"></script>',
            );
            return;
          }
          if (request.url === "/file/expired") {
            response.statusCode = 403;
            response.end();
            return;
          }
          if (request.url === "/file/whole") {
            response.writeHead(200, { "Content-Type": "video/mp4" });
            response.end(readFileSync(files.get("gray204")));
            return;
          }
          const path = files.get(request.url?.slice("/file/".length));
          if (!request.url?.startsWith("/file/") || !path) return next();
          const size = statSync(path).size;
          const range = parseByteRange(request.headers.range, size);
          if (!range || request.method !== "GET") {
            response.statusCode = 416;
            response.end();
            return;
          }
          response.writeHead(206, {
            "Content-Type": "video/mp4",
            "Content-Length": range.end - range.start + 1,
            "Content-Range": `bytes ${range.start}-${range.end}/${size}`,
          });
          const stream = createReadStream(path, range);
          response.on("close", () => stream.destroy());
          stream.pipe(response);
        });
      },
    },
  ],
});
await server.listen();
const browser = await chromium.launch({
  executablePath: option(
    "--browser",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  ),
});
try {
  const page = await browser.newPage();
  const origin = server.resolvedUrls.local[0].replace(/\/$/, "");
  await page.goto(origin + "/probe");
  await page.waitForFunction(() => !!window.videoRangeFixture);
  await page.waitForFunction(() => !!window.backgroundFixture);
  const fadeblack = await page.evaluate(() => window.backgroundFixture());
  if (!fadeblack.passed) throw new Error(JSON.stringify(fadeblack));
  const results = [];
  const refusals = [];
  for (const [id, path] of files) {
    const native = spawnSync(ffmpeg, [
      "-hide_banner",
      "-nostdin",
      "-v",
      "error",
      "-i",
      path,
      "-map",
      "0:v:0",
      "-frames:v",
      "1",
      "-f",
      "rawvideo",
      "-pix_fmt",
      "rgb24",
      "pipe:1",
    ]);
    if (native.status !== 0) throw new Error("native decode failed");
    const metadata = spawnSync(ffprobe, [
      "-v",
      "error",
      "-select_streams",
      "v:0",
      "-show_entries",
      "stream=color_space,color_transfer,color_primaries,color_range",
      "-of",
      "json",
      path,
    ]);
    const result = await page.evaluate(
      ({ url }) =>
        window.videoRangeFixture.cursor({ kind: "url", url }, "multiple", 1000),
      { url: origin + "/file/" + id },
    );
    if (id === "hdr") {
      if (
        result.error !== "CLIP_SOURCE_COLOR_UNSUPPORTED" ||
        result.decodedResources.liveDecodedFrames !== 0
      )
        throw new Error(
          "HDR was not explicitly refused/cleaned: " + JSON.stringify(result),
        );
      refusals.push({ id, ...result });
      continue;
    }
    if (result.error) throw new Error(JSON.stringify(result));
    if (
      Math.max(
        ...Array.from(native.stdout.subarray(0, 3)).map((v, c) =>
          Math.abs(v - result.centerPixel[c]),
        ),
      ) > 1 ||
      result.budget.liveFrames !== 0 ||
      result.decodedResources.liveDecodedFrames !== 0
    )
      throw new Error("Original color/resource drift");
    const decoded = spawnSync(
      ffmpeg,
      [
        "-hide_banner",
        "-nostdin",
        "-v",
        "error",
        "-i",
        path,
        "-vf",
        "trim=duration=1,fps=30",
        "-pix_fmt",
        "rgb24",
        "-f",
        "rawvideo",
        "pipe:1",
      ],
      { maxBuffer: 2 * 1024 * 1024 },
    );
    if (decoded.status !== 0) throw new Error("Native sample decode failed");
    const nativeFrames = Array.from({ length: 30 }, (_, i) =>
      Array.from(decoded.stdout.subarray(i * 64 * 32 * 3, i * 64 * 32 * 3 + 3)),
    );
    results.push({
      id,
      fingerprint: createHash("sha256")
        .update(readFileSync(path))
        .digest("hex"),
      nativeFrames,
      nativeFirstRGB: Array.from(native.stdout.subarray(0, 3)),
      nativeColorMetadata: JSON.parse(metadata.stdout.toString()),
      browser: result,
    });
  }
  const backgrounds = [];
  for (const ratio of ["vertical", "square", "horizontal"])
    for (const ids of [
      ["gray203"],
      ["gray204"],
      ["gray205"],
      ["dark"],
      ["bright"],
      ["noisy"],
      ["gray205", "untagged"],
    ]) {
      const inputs = ids.map((id) => ({
        url: origin + "/file/" + id,
        fingerprint: createHash("sha256")
          .update(readFileSync(files.get(id)))
          .digest("hex"),
      }));
      for (const transitionMs of ids.length === 1 ? [0] : [200, 300]) {
        const background = await page.evaluate(
          ({ inputs, ratio, transitionMs }) =>
            window.measureBackgroundFixture(inputs, ratio, transitionMs),
          { inputs, ratio, transitionMs },
        );
        if (background.error)
          throw new Error(
            JSON.stringify({ ids, ratio, transitionMs, ...background }),
          );
        const resources = background.diagnostics.resources;
        if (
          resources.activeCuts !== 0 ||
          resources.decoderReservedBytes !== 0 ||
          resources.presentation.liveFrames !== 0 ||
          background.diagnostics.peakRegionBytes > 24 * 1024 * 1024
        )
          throw new Error("Sampler resources escaped finite ownership");
        backgrounds.push({ ids, ratio, transitionMs, ...background });
      }
    }
  const inputs = [
    {
      url: origin + "/file/gray204",
      fingerprint: createHash("sha256")
        .update(readFileSync(files.get("gray204")))
        .digest("hex"),
    },
  ];
  const cancellation = await page.evaluate(
    ({ inputs }) =>
      window.measureBackgroundFixture(inputs, "vertical", 0, true),
    { inputs },
  );
  if (!cancellation.error)
    throw new Error("Cancelled sampler published evidence");
  for (const mode of ["expired", "whole"]) {
    const refused = await page.evaluate(
      ({ inputs, mode, origin }) =>
        window.measureBackgroundFixture(
          [{ ...inputs[0], url: origin + "/file/" + mode }],
          "vertical",
        ),
      { inputs, mode, origin },
    );
    const expected =
      mode === "expired"
        ? "CLIP_SOURCE_EXPIRED"
        : "CLIP_SOURCE_RANGE_UNSUPPORTED";
    if (
      refused.error !== expected ||
      refused.evidence ||
      refused.diagnostics.resources.presentation.liveFrames !== 0 ||
      refused.diagnostics.resources.decoderReservedBytes !== 0
    )
      throw new Error(
        "Original access refusal not fenced: " + JSON.stringify(refused),
      );
    refusals.push({ id: mode, ...refused });
  }
  const report = {
    backgrounds,
    cancellation,
    fadeblack,
    qualification: false,
    scope:
      "Actual browser original-only backgrounds and host-native decoded frame references; not release qualification",
    browser: await browser.version(),
    node: process.version,
    head: spawnSync("git", ["rev-parse", "HEAD"], { cwd: root })
      .stdout.toString()
      .trim(),
    results,
    refusals,
    sourceDirty:
      spawnSync("git", ["status", "--porcelain"], { cwd: root })
        .stdout.toString()
        .trim() !== "",
  };
  writeFileSync(
    resolve(output, "report.json"),
    JSON.stringify(report, null, 2) + "\n",
  );
  console.log(
    JSON.stringify({
      browser: report.browser,
      backgroundCases: report.backgrounds.length,
      comparisons: report.results.map((r) => ({
        id: r.id,
        native: r.nativeFirstRGB,
        browser: r.browser.centerPixel,
      })),
      qualification: false,
    }),
  );
} finally {
  await browser.close();
  await server.close();
}
