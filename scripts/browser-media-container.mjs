import { spawn } from "node:child_process";
import { existsSync, realpathSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const cache = resolve(homedir(), ".cache/ms-playwright");
const executable = chromium.executablePath();
const browser = executable.replace(cache, "/browser-cache");
if (!existsSync(executable) || !browser.startsWith("/browser-cache/"))
  throw new Error(
    "Install the locked browser first: pnpm exec playwright install chromium",
  );
const args = [
  "run",
  "--rm",
  "--network",
  "none",
  "--cpus",
  "2",
  "--memory",
  "1g",
  "--shm-size",
  "256m",
  "--user",
  `${process.getuid()}:${process.getgid()}`,
  "--mount",
  `type=bind,src=${root},dst=/work`,
  "--mount",
  `type=bind,src=${cache},dst=/browser-cache,readonly`,
  "--mount",
  `type=bind,src=${realpathSync(process.execPath)},dst=/postpilot-node,readonly`,
  "-w",
  "/work",
  "mcr.microsoft.com/playwright:v1.62.1-noble",
  "/postpilot-node",
  "scripts/browser-media-benchmark.mjs",
  "--executable",
  browser,
  "--software",
  ...process.argv.slice(2),
];
const child = spawn("docker", args, { stdio: "inherit" });
child.on("error", (error) => {
  process.stderr.write(`${error.message}\n`);
  process.exitCode = 1;
});
child.on("exit", (code, signal) => {
  process.exitCode = signal ? 1 : (code ?? 1);
});
