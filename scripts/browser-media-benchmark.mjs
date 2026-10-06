import {
  createWriteStream,
  existsSync,
  readFileSync,
  statSync,
  writeFileSync,
  mkdirSync,
} from "node:fs";
import { createReadStream } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { createRequire } from "node:module";
import { once } from "node:events";
import { chromium } from "playwright";
import {
  benchmarkReport,
  browserMediaRenderers,
  digestFile,
  parseByteRange,
  resolveFixtureFile,
  validateFixtureBundle,
} from "./browser-media-report.mjs";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRequire = createRequire(resolve(root, "frontend/package.json"));
const { createServer } = await import(
  pathToFileURL(frontendRequire.resolve("vite")).href
);
const args = process.argv.slice(2);
const option = (name, fallback) => {
  const index = args.indexOf(name);
  return index === -1 ? fallback : args[index + 1];
};
const fixtureRoot = resolve(option("--fixtures", "tmp/browser-media/native"));
const outputRoot = resolve(option("--output", "tmp/browser-media/run"));
const renderer = option("--renderer", "canvas2d-current");
if (!browserMediaRenderers.includes(renderer))
  throw new Error("Unknown benchmark renderer");
const adapterBundle = option("--adapter-bundle", "");
const bundle = validateFixtureBundle(
  JSON.parse(readFileSync(resolve(fixtureRoot, "manifest.json"), "utf8")),
);
const selected = option("--case", "").split(",").filter(Boolean);
const cases = selected.length
  ? bundle.cases.filter((value) => selected.includes(value.id))
  : bundle.cases.filter((value) => value.prepared);
if (
  !cases.length ||
  selected.some((id) => !bundle.cases.some((value) => value.id === id))
)
  throw new Error("No matching benchmark fixtures");
mkdirSync(outputRoot, { recursive: true });
const runtimeMetadata = {
  node: process.version,
  platform: process.platform,
  architecture: process.arch,
  browserLaunch: args.includes("--software")
    ? "explicit-software"
    : "default-unverified",
  competingWorkReported: args.includes("--competing-work"),
  fixtureManifestSHA256: digestFile(resolve(fixtureRoot, "manifest.json")),
  lockfileSHA256: digestFile(resolve(root, "pnpm-lock.yaml")),
  adapterBundleSHA256: adapterBundle
    ? digestFile(resolve(adapterBundle))
    : null,
  nativeProfile: bundle.nativeProfile ?? null,
  timingScope:
    "browser export only; native asset preparation is an independent recorded operation",
};
const allowed = new Set(["manifest.json"]);
for (const value of bundle.cases) {
  if (value.source?.file) allowed.add(value.source.file);
  for (const asset of value.assets ?? []) allowed.add(asset.file);
  for (const pages of Object.values(value.sheets ?? {}))
    for (const page of pages) allowed.add(page.file);
}
const server = await createServer({
  configFile: false,
  root: resolve(root, "frontend"),
  cacheDir: resolve(root, "node_modules/.cache/browser-media-vite"),
  resolve: { alias: { "@": resolve(root, "frontend/src") } },
  server: { host: "127.0.0.1", port: 0, watch: null, hmr: false },
  plugins: [
    {
      name: "private-synthetic-browser-fixtures",
      configureServer(vite) {
        vite.middlewares.use((request, response, next) => {
          if (
            request.url?.startsWith("/__browser-media-adapter__/") &&
            adapterBundle
          ) {
            try {
              const name = request.url.slice(
                "/__browser-media-adapter__/".length,
              );
              if (!/^[a-zA-Z0-9_-]+\.mjs$/.test(name))
                throw new Error("Invalid comparison chunk");
              const path = resolveFixtureFile(
                dirname(resolve(adapterBundle)),
                name,
              );
              response.setHeader("Content-Type", "text/javascript");
              createReadStream(path).pipe(response);
            } catch {
              response.statusCode = 404;
              response.end();
            }
            return;
          }
          if (!request.url?.startsWith("/__browser-media-fixtures__/"))
            return next();
          if (request.method !== "GET" && request.method !== "HEAD") {
            response.statusCode = 405;
            response.end();
            return;
          }
          try {
            const name = decodeURIComponent(
              request.url.slice("/__browser-media-fixtures__/".length),
            );
            if (!allowed.has(name)) throw new Error("Unknown fixture artifact");
            const path = resolveFixtureFile(fixtureRoot, name);
            const size = statSync(path).size;
            const range = parseByteRange(request.headers.range, size);
            response.setHeader(
              "Content-Type",
              path.endsWith(".json")
                ? "application/json"
                : path.endsWith(".png")
                  ? "image/png"
                  : "video/mp4",
            );
            response.setHeader("Accept-Ranges", "bytes");
            response.setHeader("Cache-Control", "private, max-age=60");
            response.setHeader(
              "Content-Length",
              range ? range.end - range.start + 1 : size,
            );
            if (range) {
              response.statusCode = 206;
              response.setHeader(
                "Content-Range",
                `bytes ${range.start}-${range.end}/${size}`,
              );
            }
            if (request.method === "HEAD") response.end();
            else createReadStream(path, range ?? {}).pipe(response);
          } catch {
            response.statusCode = request.headers.range ? 416 : 404;
            response.end();
          }
        });
      },
    },
  ],
});
await server.listen();
const address = server.httpServer.address();
let browser;
const reports = [];
try {
  browser = await chromium.launch({
    ...(option("--executable", "")
      ? { executablePath: option("--executable", "") }
      : {}),
    args: args.includes("--software")
      ? ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"]
      : [],
  });
  for (const fixture of cases) {
    const context = await browser.newContext();
    const activeStreams = new Map();
    try {
      const page = await context.newPage();
      page.setDefaultTimeout(240_000);
      page.on("pageerror", (error) =>
        process.stderr.write(`Fixture page failed: ${error.message}\n`),
      );
      let activeTemperature = "cold";
      await page.exposeFunction(
        "browserMediaArtifact",
        async (id, ordinal, base64) => {
          if (
            id !== fixture.id ||
            !Number.isInteger(ordinal) ||
            ordinal < 0 ||
            ordinal > 128
          )
            throw new Error("Invalid benchmark artifact handoff");
          if (base64.length > 1_500_000)
            throw new Error("Benchmark artifact chunk exceeds its bound");
          if (ordinal === 0) {
            const output = resolve(
              outputRoot,
              `${fixture.id}-${renderer}-${activeTemperature}.mp4`,
            );
            if (existsSync(output))
              throw new Error("Benchmark output already exists");
            activeStreams.set(id, {
              stream: createWriteStream(output, { flags: "wx" }),
              ordinal: 0,
            });
          }
          const state = activeStreams.get(id);
          if (!state || state.ordinal !== ordinal)
            throw new Error("Out-of-order benchmark artifact");
          state.ordinal++;
          if (!state.stream.write(Buffer.from(base64, "base64")))
            await once(state.stream, "drain");
        },
      );
      await page.goto(
        `http://127.0.0.1:${address.port}/browser-media-benchmark.html`,
      );
      await page.waitForFunction(() => window.browserMediaBenchmark?.ready);
      if (adapterBundle)
        await page.evaluate(
          () => import("/__browser-media-adapter__/adapter.mjs"),
        );
      const environment = {
        ...runtimeMetadata,
        ...(await page.evaluate(() =>
          window.browserMediaBenchmark.environment(),
        )),
      };
      for (const temperature of ["cold", "warm"]) {
        activeTemperature = temperature;
        const attemptStarted = performance.now();
        let deadline;
        const result = await Promise.race([
          page
            .evaluate(
              ({ id, renderer }) =>
                window.browserMediaBenchmark.run(id, renderer),
              { id: fixture.id, renderer },
            )
            .catch((error) => ({
              status: "failed",
              reason: error.message.slice(0, 1200),
              elapsedMs: performance.now() - attemptStarted,
            })),
          new Promise((resolve) => {
            deadline = setTimeout(
              () =>
                resolve({
                  status: "failed",
                  reason: "BROWSER_BENCHMARK_DEADLINE",
                  elapsedMs: performance.now() - attemptStarted,
                }),
              240_000,
            );
          }),
        ]).finally(() => clearTimeout(deadline));
        for (const state of activeStreams.values()) {
          state.stream.end();
          await once(state.stream, "finish");
        }
        activeStreams.clear();
        const report = benchmarkReport({
          fixture,
          renderer,
          temperature,
          environment,
          result,
        });
        reports.push(report);
        writeFileSync(
          resolve(outputRoot, `${fixture.id}-${renderer}-${temperature}.json`),
          JSON.stringify(report, null, 2) + "\n",
        );
        process.stdout.write(
          JSON.stringify({
            fixture: fixture.id,
            renderer,
            temperature,
            status: report.status,
            elapsedMs: report.elapsedMs,
            reason: report.reason,
          }) + "\n",
        );
        if (result.reason === "BROWSER_BENCHMARK_DEADLINE") break;
      }
    } finally {
      for (const state of activeStreams.values()) state.stream.destroy();
      await context.close();
    }
  }
  writeFileSync(
    resolve(outputRoot, "report.json"),
    JSON.stringify({ version: 1, reports }, null, 2) + "\n",
  );
  if (reports.some((value) => value.status === "failed")) process.exitCode = 1;
} finally {
  await browser?.close();
  await server.close();
}
