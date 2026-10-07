import assert from "node:assert/strict";
import { chromium } from "playwright";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
const url = process.env.MOBILE_WEB_URL ?? "http://127.0.0.1:2572";
const selectedRoutes = (process.env.MOBILE_ROUTES ?? "")
  .split(",")
  .filter(Boolean);
const output = resolve(
  process.env.MOBILE_OUTPUT ?? "/tmp/postpilot-mobile-t652-audit",
);
const widths = (process.env.MOBILE_WIDTHS ?? "320,360,390,430")
  .split(",")
  .map(Number);
const themes = (process.env.MOBILE_THEMES ?? "light,dark").split(",");
const html =
  '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><script type="module">import {injectIntoGlobalHook} from "/@react-refresh";injectIntoGlobalHook(window);window.$RefreshReg$=()=>{};window.$RefreshSig$=()=>(t)=>t;</script></head><body><div id="root"></div><script type="module" src="/src/test/mobile-route-fixture.tsx"></script></body></html>';
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
const receipts = [];
const inventory = [];
const findings = [];
const expectations = JSON.parse(
  await readFile(
    new URL("../frontend/src/test/mobile-audit/routes.json", import.meta.url),
    "utf8",
  ),
);
function concrete(path) {
  return (
    path
      .replace("$voiceId", "voice-default")
      .replace(
        "$templateId",
        path.startsWith("/video") ? "video-template" : "template",
      )
      .replace("$slug", "review")
      .replace("$clipId", "review")
      .replace("$testId", "test")
      .replace("$token", "gift")
      .replace("$id", "record")
      .replace("$", "materials")
      .replace(/\/$/, "") || "/"
  );
}
async function geometry(page) {
  return page.evaluate(() => {
    window.scrollTo(0, 0);
    const round = (n) => Math.round(n * 100) / 100;
    const visible = (n) =>
      !!n.getClientRects().length &&
      getComputedStyle(n).visibility !== "hidden";
    const box = (n) => {
      if (!n) return null;
      const r = n.getBoundingClientRect();
      return {
        y: round(r.top + scrollY),
        height: round(r.height),
        width: round(r.width),
      };
    };
    const main = document.querySelector("main");
    const inputs = [
      ...document.querySelectorAll(
        "input:not([type=hidden]),textarea,[contenteditable=true]",
      ),
    ].filter(visible);
    const actions = [
      ...document.querySelectorAll(
        "main button,main a[href],main [role=button]",
      ),
    ].filter(visible);
    const audit = window.__mobileAudit;
    return {
      route: audit.router.state.location.href,
      matchIds: audit.router.state.matches.map((m) => m.routeId),
      header: box(document.querySelector("header")),
      main: box(main),
      documentHeight: Math.max(
        document.body.scrollHeight,
        document.documentElement.scrollHeight,
      ),
      overflow: document.documentElement.scrollWidth > innerWidth,
      headings: [...document.querySelectorAll("h1,h2,h3")]
        .filter(visible)
        .map((n) => ({ text: n.innerText.trim(), ...box(n) })),
      firstInput: inputs[0]
        ? {
            ...box(inputs[0]),
            fontSize: getComputedStyle(inputs[0]).fontSize,
            label: inputs[0].getAttribute("aria-label") || inputs[0].id,
          }
        : null,
      firstAction: actions[0]
        ? {
            ...box(actions[0]),
            name:
              actions[0].getAttribute("aria-label") ||
              actions[0].textContent.trim(),
          }
        : null,
      inputs: inputs.map((n) => ({
        ...box(n),
        tag: n.tagName,
        type: n.type,
        fontSize: getComputedStyle(n).fontSize,
        id: n.id,
        label: n.id
          ? document.querySelector(`label[for="${n.id}"]`)?.innerText
          : n.getAttribute("aria-label"),
      })),
      alerts: [...document.querySelectorAll("[role=alert]")]
        .filter(visible)
        .map((n) => n.innerText.trim()),
      methods: [...new Set(audit.procedures)],
      body: document.body.innerText,
      controls: [
        ...document.querySelectorAll(
          "main [role=combobox],main [data-action-bar] button",
        ),
      ]
        .filter(visible)
        .map((n) => ({
          ...box(n),
          role: n.getAttribute("role"),
          name: n.getAttribute("aria-label") || n.innerText.trim(),
        })),
      theme: document.documentElement.dataset.theme,
      pointerCoarse: matchMedia("(pointer: coarse)").matches,
    };
  });
}
async function contextFor(width, theme) {
  const ctx = await browser.newContext({
    viewport: { width, height: 800 },
    isMobile: true,
    hasTouch: true,
    locale: "ko-KR",
  });
  await ctx.addInitScript((theme) => {
    localStorage.clear();
    localStorage.setItem("postpilot.theme", theme);
    localStorage.setItem("postpilot.locale", "ko");
  }, theme);
  await ctx.route("**/mobile-audit.html*", (r) =>
    r.fulfill({ contentType: "text/html", body: html }),
  );
  // Fixture inputs never call a live provider, payment gateway or private media host.
  await ctx.route(
    /^https:\/\/(private\.test|accounts\.google\.com|api\.toss)/,
    (r) => r.abort(),
  );
  return ctx;
}
async function open(page, at, state = "populated") {
  const publicRoute =
    /^\/(login|signup|forgot-password|reset-password|verify-email)(?:[/?]|$)/.test(
      at,
    );
  const fixtureKey = `${state}:${publicRoute}`;
  const reuse = await page
    .evaluate((key) => window.__mobileAudit?.fixtureKey === key, fixtureKey)
    .catch(() => false);
  if (reuse)
    await page.evaluate(
      (at) => window.__mobileAudit.router.navigate({ href: at }),
      at,
    );
  else
    await page.goto(
      `${url}/mobile-audit.html?at=${encodeURIComponent(at)}&state=${encodeURIComponent(state)}`,
      { waitUntil: "domcontentloaded" },
    );
  await page.waitForFunction(
    () => window.__mobileAudit?.router.state.status === "idle",
    { timeout: 30000 },
  );
  await page.locator("main").waitFor({ timeout: 10000 });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(150);
}

try {
  const probeContext = await contextFor(390, "light");
  const probe = await probeContext.newPage();
  await open(probe, "/");
  const registered = await probe.evaluate(
    () => window.__mobileAudit.routeInventory,
  );
  for (const r of registered) {
    const children = registered.filter((c) => c.parentId === r.id);
    inventory.push({
      ...r,
      classification:
        r.id === "__root__" || !r.path
          ? "layout"
          : children.length
            ? "layout-with-page-child"
            : !r.hasComponent
              ? "redirect"
              : /callback|verify-email|method\/(success|fail)/.test(r.fullPath)
                ? "callback"
                : "page",
      concrete: concrete(r.fullPath),
      status: "source-inspected",
    });
  }
  await probeContext.close();
  const routes = inventory.filter(
    (r) =>
      ["page", "callback", "redirect"].includes(r.classification) &&
      (!selectedRoutes.length || selectedRoutes.includes(concrete(r.fullPath))),
  );
  const unique = new Map(routes.map((r) => [r.concrete, r]));
  const jobs = [];
  for (const theme of themes)
    for (const width of widths) jobs.push({ width, theme });
  let next = 0;
  await Promise.all(
    Array.from({ length: 2 }, async () => {
      while (next < jobs.length) {
        const { width, theme } = jobs[next++];
        const ctx = await contextFor(width, theme);
        const page = await ctx.newPage();
        for (const r of unique.values()) {
          page.removeAllListeners("pageerror");
          const errors = [];
          page.on("pageerror", (e) => errors.push(e.message));
          try {
            await open(
              page,
              r.concrete === "/reset-password"
                ? "/reset-password?token=valid-audit-token"
                : r.concrete,
              r.concrete === "/setup" ? "setup" : "populated",
            );
            const g = await geometry(page);
            const expectedErrors =
              /callback|verify-email|method\/(success|fail)/.test(r.fullPath);
            const unavailable =
              errors.length ||
              /^Not Found$/.test(g.body) ||
              (/문제가 발생|다시 시도해 주세요|불러오지 못/.test(g.body) &&
                !expectedErrors);
            const receipt = {
              scenario: r.concrete === "/setup" ? "setup" : "populated",
              id: r.id,
              path: r.fullPath,
              at: r.concrete,
              width,
              theme,
              status: unavailable ? "unavailable" : "checked",
              ...g,
              errors,
            };
            assert.ok(g.main);
            assert.equal(
              g.pointerCoarse,
              true,
              "Phone emulation retains native coarse pointer",
            );
            assert.equal(
              g.overflow,
              false,
              `Horizontal overflow ${r.concrete}`,
            );
            for (const input of g.inputs)
              if (
                input.type !== "checkbox" &&
                input.type !== "radio" &&
                input.type !== "range" &&
                input.type !== "file"
              )
                assert.ok(
                  parseFloat(input.fontSize) >= 16,
                  `${r.concrete} input font ${input.fontSize}`,
                );
            const expected = expectations[r.concrete];
            assert.ok(
              expected,
              `No expectation for registered path ${r.concrete}`,
            );
            if (!unavailable) {
              assert.equal(g.route.split("?")[0], expected.expectedPath);
              assert.equal(g.matchIds.at(-1), expected.expectedLeaf);
              if (expected.heading)
                assert.ok(
                  g.headings.some(
                    (h) =>
                      h.text.replace(/\s+/g, "") ===
                      expected.heading.replace(/\s+/g, ""),
                  ),
                  `Expected ${expected.heading} on ${r.concrete}: ${g.headings.map((h) => h.text).join(" / ")}`,
                );
              if (expected.control) {
                const control = page.getByRole(expected.control.role, {
                  name: expected.control.name,
                  exact: true,
                });
                assert.equal(await control.count(), 1);
                assert.equal(
                  await control.getAttribute("aria-selected"),
                  "true",
                );
              }
              if (expected.body)
                assert.ok(
                  g.body.includes(expected.body),
                  `Expected named form ${expected.body}`,
                );
            }
            receipt.assertedState = unavailable
              ? "Fixture or runtime unavailable"
              : r.classification === "redirect"
                ? `Redirected to ${g.route}`
                : `Mounted ${g.matchIds.at(-1)} with ${g.headings.map((h) => h.text).join(" / ")}`;
            receipts.push(receipt);
            if (
              [
                "/",
                "/settings",
                "/tests",
                "/admin/vouchers",
                "/admin/models",
                "/login",
                "/signup",
                "/forgot-password",
                "/reset-password",
                "/account",
                "/billing",
              ].includes(r.concrete)
            ) {
              const field = page
                .locator(
                  "input:not([type=hidden]):not([type=checkbox]):not([type=radio]):not([type=range]):not([readonly]),textarea",
                )
                .first();
              if (await field.count()) {
                const type = await field.getAttribute("type");
                if (type !== "number")
                  await field.fill("모바일에서 이어 쓰는 내용");
                await field.evaluate((n) => {
                  n.focus();
                  try {
                    n.setSelectionRange(3, 3);
                  } catch {}
                  window.__mobileNode = n;
                });
                const capture = () =>
                  field.evaluate((n) => ({
                    value: n.value,
                    start: n.selectionStart,
                    end: n.selectionEnd,
                    focused: document.activeElement === n,
                    sameNode: window.__mobileNode === n,
                  }));
                const before = await capture();
                const measurements = [];
                for (const viewport of [
                  { width: 1440, height: 900 },
                  { width, height: 480 },
                  { width, height: 800 },
                ]) {
                  await page.setViewportSize(viewport);
                  await page.waitForTimeout(80);
                  assert.deepEqual(
                    await capture(),
                    before,
                    `Draft and caret survive width/keyboard changes on ${r.concrete}`,
                  );
                  const reflow = await geometry(page);
                  assert.equal(reflow.overflow, false);
                  measurements.push({
                    viewport,
                    header: reflow.header,
                    main: reflow.main,
                    firstInput: reflow.firstInput,
                    documentHeight: reflow.documentHeight,
                  });
                }
                receipt.continuity = { pass: true, before, measurements };
              } else {
                await page.setViewportSize({ width: 1440, height: 900 });
                const desktop = await geometry(page);
                assert.equal(desktop.overflow, false);
                receipt.desktop = {
                  header: desktop.header,
                  main: desktop.main,
                  documentHeight: desktop.documentHeight,
                };
                await page.setViewportSize({ width, height: 800 });
              }
            }

            // Full-page captures can change Chromium's native pointer media query.
            // Capture only after phone geometry and continuity checks, then close this page.
            if (
              width === 390 &&
              theme === "light" &&
              [
                "/",
                "/settings",
                "/tests",
                "/admin/vouchers",
                "/admin/models",
                "/login",
                "/signup",
                "/forgot-password",
                "/reset-password",
                "/account",
                "/billing",
              ].includes(r.concrete)
            ) {
              const shotContext = await contextFor(width, theme);
              const shotPage = await shotContext.newPage();
              await open(
                shotPage,
                r.concrete,
                r.concrete === "/setup" ? "setup" : "populated",
              );
              await shotPage.screenshot({
                path: `${output}/${r.concrete.replaceAll("/", "_") || "home"}.png`,
                fullPage: true,
              });
              await shotContext.close();
            }
            console.log(
              JSON.stringify({
                at: r.concrete,
                width,
                theme,
                status: receipt.status,
                height: g.documentHeight,
                inputY: g.firstInput?.y,
                headings: g.headings.map((h) => h.text),
                alerts: g.alerts,
                overflow: g.overflow,
              }),
            );
          } catch (e) {
            findings.push({
              path: r.fullPath,
              width,
              theme,
              error: String(e),
              errors,
            });
            console.log(JSON.stringify(findings.at(-1)));
          }
          await writeFile(
            `${output}/receipts.json`,
            JSON.stringify({ inventory, receipts, findings }, null, 2),
          );
        }
        await ctx.close();
      }
    }),
  );
  await writeFile(
    `${output}/receipts.json`,
    JSON.stringify({ inventory, receipts, findings }, null, 2),
  );
  // Distinct data/input/operation states are additional evidence, never substituted for the
  // populated route matrix. Browser data remain local deterministic RPC fixtures.
  const stateCases = [
    ...[
      "/posts",
      "/clips",
      "/templates",
      "/video-templates",
      "/guidelines",
      "/video-guidelines",
      "/memories",
      "/voices",
      "/spoken-voices",
      "/billing",
      "/tests/history",
    ].map((at) => ({ at, state: "empty", kind: "empty" })),
    ...[
      "/posts",
      "/clips",
      "/templates",
      "/video-templates",
      "/guidelines",
      "/memories",
      "/voices",
      "/spoken-voices",
      "/billing",
      "/ai-models",
      "/admin/models",
      "/admin/estimator",
      "/admin/vouchers",
      "/admin/costs",
      "/posts/review",
      "/clips/review",
      "/templates/template",
      "/video-templates/video-template",
    ].map((at) => ({ at, state: "error", kind: "error" })),
    ...["/posts/review", "/clips/review"].map((at) => ({
      at,
      state: "draft",
      kind: "draft",
    })),
    ...[
      "/posts/review",
      "/clips/review",
      "/posts",
      "/clips",
      "/tests/test",
      "/tests/history",
    ].flatMap((at) =>
      ["working", "failed"].map((state) => ({ at, state, kind: state })),
    ),
    ...["/posts/review", "/clips/review"].map((at) => ({
      at,
      state: "finalized",
      kind: "finalized",
    })),
    {
      at: "/billing/checkout?tier=pro&term=monthly",
      state: "checkout",
      kind: "checkout",
    },
    {
      at: "/reset-password?token=expired-audit-token",
      state: "invalid-reset",
      kind: "invalid-reset",
    },
    {
      at: "/verify-email?token=expired-audit-token",
      state: "invalid-verification",
      kind: "error",
    },
    { at: "/clips/review", state: "draft", kind: "design-gallery" },
    { at: "/admin/models", state: "catalog-populated", kind: "catalog" },
    {
      at: "/admin/estimator",
      state: "catalog-populated",
      kind: "catalog-estimator",
    },
    ...[2, 4, 8, 16].map((count) => ({
      at: `/tests?count=${count}&draft=audit-count-${count}`,
      state: "populated",
      kind: "candidates",
      count,
    })),
    { at: "/templates/template", state: "populated", kind: "direct-template" },
    {
      at: "/video-templates/video-template",
      state: "populated",
      kind: "direct-template",
    },
  ];
  for (const theme of themes) {
    const ctx = await contextFor(390, theme);
    const page = await ctx.newPage();
    for (const c of stateCases) {
      if (selectedRoutes.length && !selectedRoutes.includes(c.at.split("?")[0]))
        continue;
      page.removeAllListeners("pageerror");
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      try {
        await open(page, c.at, c.state);
        const basePath = c.at.split("?")[0];
        const expected = expectations[basePath];
        if (c.kind === "candidates") {
          await page
            .getByRole("button", { name: "AI 모델", exact: true })
            .click();
          await page
            .getByRole("combobox", { name: /후보|참가/ })
            .first()
            .waitFor();
          const choices = page.getByRole("combobox", { name: /후보|참가/ });
          assert.equal(await choices.count(), c.count);
        }
        if (c.kind === "invalid-reset") {
          await page.locator("input[type=password]").fill("password-audit");
          await page
            .getByRole("button", { name: "비밀번호 재설정", exact: true })
            .click();
          await page.getByRole("alert").waitFor();
        }
        if (c.kind === "direct-template") {
          await page.getByRole("button", { name: /직접 편집하기/ }).click();
          await page
            .getByLabel(
              basePath.startsWith("/video-") ? "템플릿 이름" : "이름",
              { exact: true },
            )
            .waitFor();
        }
        const closed =
          c.kind === "design-gallery" ? await geometry(page) : undefined;
        let chosenDesign;
        if (c.kind === "design-gallery") {
          const toggle = page.getByRole("button", {
            name: "디자인과 자막 스타일",
            exact: true,
          });
          assert.equal(await toggle.getAttribute("aria-expanded"), "false");
          await toggle.click();
          await page
            .getByRole("radiogroup", { name: "인트로 디자인", exact: true })
            .waitFor();
          const intro = page.getByRole("radiogroup", {
            name: "인트로 디자인",
            exact: true,
          });
          await intro
            .getByRole("radio", { name: "매거진 커버", exact: true })
            .click();
          chosenDesign = await intro
            .getByRole("radio", { checked: true })
            .innerText();
          await toggle.click();
          assert.equal(await toggle.getAttribute("aria-expanded"), "false");
          assert.ok(
            (await page.locator("main").innerText()).includes(chosenDesign),
          );
          const field = page.getByLabel("클립 제목", { exact: true });
          await field.fill("크기를 바꾸어도 유지되는 클립 제목");
          await field.evaluate((n) => {
            n.focus();
            n.setSelectionRange(5, 5);
            window.__mobileNode = n;
          });
          const capture = () =>
            field.evaluate((n) => ({
              value: n.value,
              start: n.selectionStart,
              end: n.selectionEnd,
              sameNode: window.__mobileNode === n,
              focused: document.activeElement === n,
            }));
          const before = await capture();
          for (const viewport of [
            { width: 1440, height: 900 },
            { width: 390, height: 480 },
            { width: 390, height: 800 },
          ]) {
            await page.setViewportSize(viewport);
            await page.waitForTimeout(100);
            assert.deepEqual(await capture(), before);
            assert.equal(
              await toggle.getAttribute("aria-expanded"),
              viewport.width >= 768 ? "true" : "false",
            );
            if (viewport.width >= 768) {
              await toggle.click();
              assert.equal(await toggle.getAttribute("aria-expanded"), "false");
              await field.focus();
              assert.deepEqual(await capture(), before);
            }
            assert.equal((await geometry(page)).overflow, false);
          }
          await toggle.click();
          assert.equal(
            await intro
              .getByRole("radio", { name: chosenDesign, exact: true })
              .getAttribute("aria-checked"),
            "true",
          );
          await toggle.click();
          assert.ok(
            !(await page.evaluate(() => window.__mobileAudit.procedures)).some(
              (name) => name.startsWith("Start"),
            ),
            "Opening/selecting/resizing design never starts AI",
          );
        }
        const g = await geometry(page);
        assert.equal(errors.length, 0, errors.join("; "));
        assert.equal(g.overflow, false);
        assert.equal(g.matchIds.at(-1), expected.expectedLeaf);
        if (c.kind === "error" || c.kind === "invalid-reset")
          assert.ok(
            g.alerts.length || /불러오지 못|오류|연결/.test(g.body),
            `Expected mounted error on ${c.at}`,
          );
        if (c.kind === "catalog")
          assert.ok(
            (await page.getByRole("listitem").count()) > 0 &&
              g.body.includes("provider-"),
            "Populated catalog rows mount",
          );
        if (c.kind === "catalog-estimator")
          assert.equal(
            await page.locator("main [role=combobox]").count(),
            8,
            "Every tier keeps observer/writer controls",
          );
        if (c.kind === "checkout") {
          assert.ok(g.body.includes("Pro"));
          assert.ok(g.methods.includes("QuotePrice"));
          assert.ok(g.body.includes("결제 금액"));
        }
        if (c.kind === "draft")
          assert.ok(g.firstInput, `Expected draft input on ${c.at}`);
        if (c.kind === "working")
          assert.ok(
            g.body.includes(
              basePath.startsWith("/tests")
                ? "후보별 글을 만들고 있어요"
                : "작성 중",
            ),
            `Expected actual named operation on ${c.at}`,
          );
        if (c.kind === "failed")
          assert.ok(
            basePath.startsWith("/tests")
              ? /일부 글을 만들지 못|요청을 완료하지 못/.test(g.body)
              : g.body.includes("AI 모델을 잠시 사용할 수 없어요."),
            `Expected actual named failure on ${c.at}`,
          );
        if (c.kind === "finalized") {
          const tab = page.getByRole("tab", {
            name: basePath.startsWith("/clips") ? "완성" : "글 완성",
            exact: true,
          });
          assert.equal(await tab.getAttribute("aria-selected"), "true");
          if (basePath.startsWith("/clips"))
            assert.equal(
              await page
                .getByRole("link", { name: "영상 다운로드", exact: true })
                .count(),
              1,
            );
          else assert.ok(g.body.includes("내보내기"));
        }
        const receipt = {
          id: expected.expectedLeaf,
          path: basePath,
          at: c.at,
          width: 390,
          theme,
          scenario: c.state,
          interaction: c.kind,
          status: "checked",
          assertedState: c.kind,
          design: closed
            ? {
                closedHeight: closed.documentHeight,
                selected: chosenDesign,
                closedInputs: closed.inputs,
                continuity: true,
              }
            : undefined,
          ...g,
          errors,
        };
        if (c.kind === "direct-template") {
          const input = page.getByLabel(
            basePath.startsWith("/video-") ? "템플릿 이름" : "이름",
            { exact: true },
          );
          await input.fill("화면 크기와 무관하게 유지되는 이름");
          await input.evaluate((n) => {
            n.focus();
            n.setSelectionRange(5, 5);
            window.__mobileNode = n;
          });
          const capture = () =>
            input.evaluate((n) => ({
              value: n.value,
              start: n.selectionStart,
              focused: document.activeElement === n,
              sameNode: window.__mobileNode === n,
            }));
          const before = await capture();
          const measurements = [];
          for (const viewport of [
            { width: 1440, height: 900 },
            { width: 390, height: 480 },
            { width: 390, height: 800 },
          ]) {
            await page.setViewportSize(viewport);
            await page.waitForTimeout(100);
            assert.deepEqual(await capture(), before);
            const reflow = await geometry(page);
            assert.equal(reflow.overflow, false);
            measurements.push({
              viewport,
              firstInput: reflow.firstInput,
              documentHeight: reflow.documentHeight,
            });
          }
          receipt.continuity = { pass: true, before, measurements };
        }
        receipts.push(receipt);
        console.log(
          JSON.stringify({
            at: c.at,
            state: c.state,
            interaction: c.kind,
            theme,
            status: "checked",
            height: g.documentHeight,
            inputY: g.firstInput?.y,
            alerts: g.alerts,
          }),
        );
      } catch (e) {
        findings.push({
          at: c.at,
          state: c.state,
          interaction: c.kind,
          theme,
          error: String(e),
          errors,
        });
        console.log(JSON.stringify(findings.at(-1)));
      }
      await writeFile(
        `${output}/receipts.json`,
        JSON.stringify({ inventory, receipts, findings }, null, 2),
      );
    }
    await ctx.close();
  }
  console.log(
    JSON.stringify({
      registered: inventory.length,
      paths: unique.size,
      receipts: receipts.length,
      unavailable: receipts.filter((r) => r.status === "unavailable").length,
      findings: findings.length,
      output,
    }),
  );
  if (findings.length) process.exitCode = 1;
} finally {
  await browser.close();
}
