import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
import { chromium } from 'playwright'

const baseUrl = process.env.T624_WEB_URL ?? 'http://127.0.0.1:26124'
const output = process.env.T624_HISTORY_BROWSER_OUTPUT ?? '/tmp/t624-history-return'
const ownerId = 't624-browser'
const targetId = 't624-history-target'
const query = '여행'
const status = 'review'
const savedScroll = 900
const storageKey = `postpilot.return.${ownerId}`
const entry = {
  version: 1,
  ownerKey: ownerId,
  path: '/posts',
  section: 'posts',
  filters: { q: query, status },
  scrollY: savedScroll,
  targetId,
}
const rows = Array.from({ length: 40 }, (_, index) => ({
  slug: index === 34 ? targetId : `t624-history-${index + 1}`,
  title: `${query} 기록 ${String(index + 1).padStart(2, '0')}`,
  status,
  targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
  updatedAt: '2026-10-07T00:00:00Z',
  tags: [],
  contentReady: false,
  exportReady: false,
}))
const post = {
  slug: targetId,
  title: rows[34].title,
  memo: '필터와 읽던 위치를 유지합니다.',
  status,
  targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
  createdAt: '2026-10-07T00:00:00Z',
  updatedAt: '2026-10-07T00:00:00Z',
}
const deferred = () => {
  let resolve
  const promise = new Promise((done) => {
    resolve = done
  })
  return { promise, resolve }
}
const firstSeen = deferred()
const firstGate = deferred()
const secondSeen = deferred()
const secondGate = deferred()
const calls = []
const requests = []
const errors = []
const evidence = {
  baseUrl,
  viewport: { width: 1440, height: 900 },
  entry,
  calls,
  requests,
  errors,
  phases: [],
}
await mkdir(output, { recursive: true })
const browser = await chromium.launch({ headless: true })
try {
  const context = await browser.newContext({ viewport: evidence.viewport, locale: 'ko-KR' })
  await context.addInitScript(
    ({ ownerId, storageKey, entry, targetId }) => {
      localStorage.setItem('postpilot.theme', 'light')
      localStorage.setItem('postpilot.locale', 'ko')
      localStorage.setItem(
        `postpilot.setup.v1.${ownerId}`,
        JSON.stringify({
          version: 1,
          ownerId,
          completed: true,
          skipped: [],
          resume: 'welcome',
          target: '/',
        }),
      )
      sessionStorage.setItem(storageKey, JSON.stringify(entry))
      sessionStorage.setItem(
        `${storageKey}.post.${encodeURIComponent(targetId)}`,
        JSON.stringify(entry),
      )
      window.__historyReturnScrollCalls = []
      const scrollTo = window.scrollTo.bind(window)
      window.scrollTo = (...args) => {
        const context = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
        window.__historyReturnScrollCalls.push({
          at: performance.now(),
          args,
          pathname: location.pathname,
          targetLoaded: Boolean(document.querySelector(`main a[href="/posts/${targetId}"]`)),
          documentHeight: document.documentElement.scrollHeight,
          beforeY: window.scrollY,
          restoreFlag: context?.filters.restoreScroll ?? null,
        })
        scrollTo(...args)
      }
    },
    { ownerId, storageKey, entry, targetId },
  )
  const page = await context.newPage()
  page.on('pageerror', (error) => errors.push(error.message))
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    // Source-module paths such as /src/shared/api/... must reach Vite unchanged.
    if (!pathname.startsWith('/api/')) return route.continue()
    const method = pathname.split('/').at(-1)
    calls.push(method)
    let response = {}
    if (method === 'GetMe')
      response = {
        user: {
          id: ownerId,
          email: 'browser@example.test',
          emailVerified: true,
          hasPassword: true,
        },
        plan: 'PLAN_FREE',
      }
    if (method === 'GetMyPlan') response = { plan: 'PLAN_FREE', balance: { credits: 0, lots: [] } }
    if (method === 'GetPost') response = { post }
    if (method === 'ListPosts') {
      const body = request.postDataJSON()
      const captured = { at: Date.now(), ...body }
      requests.push(captured)
      assert.equal(body.query, query)
      assert.equal(body.status, status)
      assert.equal(body.pageSize, 20)
      if (!body.pageToken) {
        firstSeen.resolve()
        await firstGate.promise
        response = { posts: rows.slice(0, 20), nextPageToken: 'page-2' }
      } else {
        assert.equal(body.pageToken, 'page-2')
        secondSeen.resolve()
        await secondGate.promise
        response = { posts: rows.slice(20), nextPageToken: '' }
      }
      captured.releasedAt = Date.now()
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(response),
    })
  })
  const measure = async (phase) => {
    const reading = await page.evaluate(
      ({ storageKey, targetId }) => {
        const current = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
        const scoped = JSON.parse(
          sessionStorage.getItem(`${storageKey}.post.${encodeURIComponent(targetId)}`) ?? 'null',
        )
        const slugs = [
          ...new Set(
            [...document.querySelectorAll('main a[href^="/posts/"]')].map((node) =>
              node.getAttribute('href'),
            ),
          ),
        ]
        return {
          path: location.pathname,
          search: location.search,
          scrollY: window.scrollY,
          maximumScroll: document.documentElement.scrollHeight - innerHeight,
          loadedRows: slugs.length,
          targetLoaded: slugs.includes(`/posts/${targetId}`),
          restoreFlag: current?.filters.restoreScroll ?? null,
          scopedEntry: scoped,
          scrollCalls: window.__historyReturnScrollCalls,
        }
      },
      { storageKey, targetId },
    )
    evidence.phases.push({ phase, at: Date.now(), ...reading })
    return reading
  }
  await page.goto(`${baseUrl}/posts/${targetId}`, { waitUntil: 'commit', timeout: 60000 })
  const back = page.getByRole('link', { name: '작업 내역으로 돌아가기', exact: true })
  await back.waitFor({ state: 'visible', timeout: 60000 })
  assert.equal(requests.length, 0, 'opening the source editor must not warm the history list')
  await measure('source-editor-cold-list')
  await back.click()
  await page.waitForURL((url) => url.pathname === '/posts', { timeout: 60000 })
  await Promise.race([
    firstSeen.promise,
    new Promise((_, reject) =>
      setTimeout(() => reject(Error('initial history request missing')), 30000),
    ),
  ])
  await page.waitForTimeout(350)
  const initialPending = await measure('initial-list-response-delayed')
  assert.equal(initialPending.loadedRows, 0)
  assert.equal(initialPending.scrollY, 0)
  assert.equal(initialPending.restoreFlag, 'true')
  firstGate.resolve()
  await Promise.race([
    secondSeen.promise,
    new Promise((_, reject) =>
      setTimeout(() => reject(Error('second history page request missing')), 30000),
    ),
  ])
  await page.waitForFunction(
    () => document.querySelectorAll('main a[href^="/posts/"]').length >= 20,
  )
  await page.waitForTimeout(350)
  const secondPending = await measure('page-two-response-delayed')
  assert.equal(secondPending.loadedRows, 20)
  assert.equal(secondPending.targetLoaded, false)
  assert.ok(
    secondPending.maximumScroll < savedScroll,
    'the first page cannot reach the retained position',
  )
  assert.ok(secondPending.scrollY < savedScroll)
  assert.equal(secondPending.restoreFlag, 'true')
  secondGate.resolve()
  await page.waitForFunction(
    ({ storageKey, targetId, savedScroll }) => {
      const entry = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
      return (
        Boolean(document.querySelector(`main a[href="/posts/${targetId}"]`)) &&
        Math.abs(window.scrollY - savedScroll) < 1 &&
        entry?.filters.restoreScroll === undefined
      )
    },
    { storageKey, targetId, savedScroll },
    { timeout: 30000 },
  )
  const restored = await measure('second-page-painted-and-scroll-restored')
  assert.equal(restored.loadedRows, 40)
  assert.equal(restored.targetLoaded, true)
  assert.equal(restored.scrollY, savedScroll)
  assert.equal(restored.restoreFlag, null)
  assert.equal(restored.scopedEntry.scrollY, savedScroll)
  const parameters = new URLSearchParams(restored.search)
  assert.equal(parameters.get('q'), query)
  assert.equal(parameters.get('status'), status)
  // The Vite development tree may replay the initial read under React Strict Mode.
  // Both delayed page classes must be requested; a read replay is not another history page.
  assert.deepEqual([...new Set(requests.map((request) => request.pageToken ?? ''))], ['', 'page-2'])
  assert.ok(restored.scrollCalls.some((call) => call.targetLoaded && call.args[1] === savedScroll))
  assert.deepEqual(errors, [])
  await page.screenshot({ path: `${output}/restored-history.png`, fullPage: true })
  evidence.passed = true
  console.log(
    JSON.stringify(
      {
        passed: true,
        requests: requests.length,
        loadedRows: restored.loadedRows,
        scrollY: restored.scrollY,
        restoreFlag: restored.restoreFlag,
        output,
      },
      null,
      2,
    ),
  )
  await context.close()
} catch (error) {
  evidence.passed = false
  evidence.failure = String(error.stack ?? error)
  throw error
} finally {
  firstGate.resolve()
  secondGate.resolve()
  await writeFile(`${output}/measurements.json`, JSON.stringify(evidence, null, 2))
  await browser.close()
}
