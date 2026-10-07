import { chromium } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
import assert from 'node:assert/strict'
const dir = process.env.T624_BROWSER_OUTPUT ?? '/tmp/t624-browser'
await mkdir(dir, { recursive: true })
const browser = await chromium.launch({ headless: true })
const results = []
try {
  for (const theme of ['light', 'dark']) {
    for (const viewport of [
      { width: 1440, height: 900 },
      { width: 1920, height: 1080 },
      { width: 360, height: 800 },
      { width: 320, height: 640 },
      { width: 720, height: 450 },
    ]) {
      const context = await browser.newContext({
        viewport,
        locale: 'ko-KR',
        isMobile: viewport.width <= 430,
        hasTouch: viewport.width <= 430,
      })
      await context.addInitScript((theme) => {
        localStorage.setItem('postpilot.theme', theme)
        localStorage.setItem('postpilot.locale', 'ko')
        localStorage.setItem(
          'postpilot.setup.v1.t624-browser',
          JSON.stringify({
            version: 1,
            ownerId: 't624-browser',
            completed: true,
            skipped: [],
            resume: 'welcome',
            target: '/',
          }),
        )
      }, theme)
      const page = await context.newPage()
      const errors = []
      const calls = []
      page.on('pageerror', (e) => errors.push(e.message))
      let post = {
        slug: 't624-saved',
        title: '',
        memo: '',
        status: 'draft',
        targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
        createdAt: '2026-10-07T00:00:00Z',
        updatedAt: '2026-10-07T00:00:00Z',
      }
      await page.route('**/api/**', async (route) => {
        const req = route.request()
        const path = new URL(req.url()).pathname
        if (!path.startsWith('/api/')) return route.continue()
        const method = path.split('/').at(-1)
        calls.push(method)
        let response = {}
        if (method === 'GetMe')
          response = {
            user: {
              id: 't624-browser',
              email: 'browser@example.test',
              emailVerified: true,
              hasPassword: true,
            },
            plan: 'PLAN_FREE',
          }
        if (method === 'GetMyPlan')
          response = { plan: 'PLAN_FREE', balance: { credits: 0, lots: [] } }
        if (method === 'SavePostDraft') {
          const body = req.postDataJSON()
          post = {
            slug: body.slug || 't624-minted',
            title: body.title ?? post?.title ?? '',
            memo: body.memo ?? post?.memo ?? '',
            status: 'draft',
            targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
            createdAt: '2026-10-07T00:00:00Z',
            updatedAt: '2026-10-07T00:00:00Z',
          }
          response = { post }
        }
        if (method === 'GetPost') response = { post }
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(response),
        })
      })
      await page.goto(`${process.env.T624_WEB_URL ?? 'http://127.0.0.1:26124'}/posts/t624-saved`, {
        waitUntil: 'commit',
        timeout: 60000,
      })
      try {
        await page.locator('#post-memo').waitFor({ state: 'visible', timeout: 60000 })
      } catch (e) {
        console.log(JSON.stringify({ body: await page.locator('body').innerText(), errors, calls }))
        throw e
      }
      await page.evaluate(() => document.fonts.ready)
      const measure = () =>
        page.evaluate(() => {
          const rect = (n) => {
            const r = n.getBoundingClientRect()
            return { x: r.x, y: r.y, width: r.width, height: r.height, bottom: r.bottom }
          }
          const memo = document.querySelector('#post-memo')
          return {
            main: rect(document.querySelector('main')),
            memo: {
              ...rect(memo),
              font: getComputedStyle(memo).fontSize,
              overflow: getComputedStyle(memo).overflowY,
            },
            overflow: document.documentElement.scrollWidth > innerWidth,
            scrollContainers: [...document.querySelectorAll('main *')].filter((n) => {
              const s = getComputedStyle(n)
              return ['auto', 'scroll'].includes(s.overflowY) && n.scrollHeight > n.clientHeight
            }).length,
          }
        })
      const empty = await measure()
      assert.equal(empty.overflow, false)
      assert.equal(empty.scrollContainers, 0)
      const memo = page.locator('#post-memo')
      await memo.fill('짧은 메모')
      await memo.evaluate((n) => n.setSelectionRange(3, 3))
      const typed = await measure()
      assert.equal(typed.overflow, false)
      assert.equal(typed.scrollContainers, 0)
      if (viewport.width <= 360) assert.ok(parseFloat(typed.memo.font) >= 16)
      await page.setViewportSize({ width: viewport.width, height: viewport.height + 100 })
      const caret = await memo.evaluate((n) => ({
        value: n.value,
        start: n.selectionStart,
        end: n.selectionEnd,
        focused: document.activeElement === n,
      }))
      assert.deepEqual(caret, { value: '짧은 메모', start: 3, end: 3, focused: true })
      await page.setViewportSize(viewport)
      await page.screenshot({
        path: dir + '/' + viewport.width + '-' + theme + '.png',
        fullPage: true,
      })
      results.push({ theme, viewport, empty, typed, caret, errors })
      await writeFile(dir + '/measurements.json', JSON.stringify(results, null, 2))
      assert.deepEqual(errors, [])
      await context.close()
    }
  }
  for (const theme of ['light', 'dark']) {
    const a = results.find((r) => r.theme === theme && r.viewport.width === 1440)
    const b = results.find((r) => r.theme === theme && r.viewport.width === 1920)
    assert.ok(a.empty.main.width > 768)
    assert.ok(b.empty.main.width >= a.empty.main.width)
    assert.ok(b.empty.memo.height > a.empty.memo.height + 100)
  }
  await writeFile(dir + '/measurements.json', JSON.stringify(results, null, 2))
  console.log(
    JSON.stringify(
      results.map(({ theme, viewport, empty }) => ({
        theme,
        viewport,
        workspaceWidth: empty.main.width,
        memoHeight: empty.memo.height,
        overflow: empty.overflow,
        scrollContainers: empty.scrollContainers,
      })),
      null,
      2,
    ),
  )
} finally {
  await browser.close()
}
