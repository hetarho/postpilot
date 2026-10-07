import { createRequire } from "node:module";
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  createReadStream,
} from "node:fs";
import { resolve, basename } from "node:path";
import { pathToFileURL } from "node:url";
import { chromium } from "playwright";
const root = resolve(import.meta.dirname, ".."),
  args = process.argv.slice(2);
const option = (key, fallback) =>
  args.includes(key) ? args[args.indexOf(key) + 1] : fallback;
const fixtures = resolve(option("--fixtures", "tmp/browser-ink-native")),
  output = resolve(option("--output", "tmp/browser-ink-run"));
const manifest = JSON.parse(
  readFileSync(resolve(fixtures, "manifest.json"), "utf8"),
);
const selected = option("--case", "").split(",").filter(Boolean),
  cases = selected.length
    ? manifest.cases.filter((c) => selected.includes(c.id))
    : manifest.cases;
if (
  !cases.length ||
  selected.some((id) => !cases.some((fixture) => fixture.id === id))
)
  throw new Error("Requested static ink fixture is missing");
const frontendRequire = createRequire(resolve(root, "frontend/package.json"));
const { createServer } = await import(
  pathToFileURL(frontendRequire.resolve("vite")).href
);
const allowed = new Set(cases.map((c) => c.reference));
const server = await createServer({
  configFile: false,
  root: resolve(root, "frontend"),
  resolve: { alias: { "@": resolve(root, "frontend/src") } },
  server: {
    host: "127.0.0.1",
    port: 0,
    watch: null,
    headers: {
      "Cross-Origin-Opener-Policy": "same-origin",
      "Cross-Origin-Embedder-Policy": "require-corp",
    },
  },
  plugins: [
    {
      name: "synthetic-ink-references",
      configureServer(vite) {
        vite.middlewares.use((request, response, next) => {
          if (!request.url?.startsWith("/__ink-fixtures__/")) return next();
          const name = request.url.slice("/__ink-fixtures__/".length);
          if (!allowed.has(name) || basename(name) !== name) {
            response.writeHead(404);
            response.end();
            return;
          }
          response.setHeader("Content-Type", "image/png");
          createReadStream(resolve(fixtures, name)).pipe(response);
        });
      },
    },
  ],
});
await server.listen();
mkdirSync(output, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: option(
    "--chrome",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  ),
});
try {
  const page = await browser.newPage();
  await page.goto(
    `http://127.0.0.1:${server.httpServer.address().port}/browser-ink-fixtures.html`,
  );
  await page.waitForFunction(() => !!window.browserInkFixtures);
  const environment = await page.evaluate(() => {
    const c = document.createElement("canvas"),
      gl = c.getContext("webgl2"),
      ext = gl?.getExtension("WEBGL_debug_renderer_info");
    const renderer = ext ? gl.getParameter(ext.UNMASKED_RENDERER_WEBGL) : null;
    gl?.getExtension("WEBGL_lose_context")?.loseContext();
    return {
      userAgent: navigator.userAgent,
      renderer,
      crossOriginIsolated,
      hardwareConcurrency: navigator.hardwareConcurrency,
    };
  });
  const results = [];
  for (const fixture of cases) {
    const result = await page.evaluate(
      (f) => f.stress ? window.browserInkFixtures.stress(f,f.renderer) : window.browserInkFixtures.run(f),
      { ...fixture, renderer: option("--renderer", "canvas"), stress: args.includes("--stress") },
    );
    const { png, ...summary } = result;
    if (png) writeFileSync(
      resolve(output, fixture.id + ".png"),
      Buffer.from(png, "base64"),
    );
    results.push(summary);
    console.log(JSON.stringify(summary));
  }
  writeFileSync(
    resolve(output, "report.json"),
    JSON.stringify(
      {
        version: 1,
        productionQualified: false,
        competingWork: true,
        nativeProfile: manifest.native,
        environment,
        results,
      },
      null,
      2,
    ) + "\n",
  );
} finally {
  await browser.close();
  await server.close();
}
