import { chromium } from 'playwright'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import assert from 'node:assert/strict'

const output = process.env.T643_BROWSER_OUTPUT ?? '/tmp/postpilot-t643-browser'
const url = process.env.T643_WEB_URL ?? 'http://127.0.0.1:26143'
await mkdir(output, { recursive: true })
const browser = await chromium.launch({ headless: true })
const results = []
const ownerId = 't643-browser'
const post = {
  slug: 't643-inspection',
  status: 'published',
  title: '소유자가 고른 글',
  memo: '저장된 자료',
  inputRevision: '3',
  contentRevision: '2',
  contentHash: 'synthetic-result',
  machineBaselineRevision: '2',
  finalizedRevision: '2',
  publishedUrl: 'https://example.test/post',
  targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
  contentLanguage: 'CONTENT_LANGUAGE_KOREAN',
  tagCount: 4,
  content: {
    title: '정식 글 제목',
    summary: '정식 요약',
    tags: ['정식태그'],
    blocks: [{ type: 'TEXT', content: '정식 본문은 기술 복사에 포함되지 않습니다.' }],
  },
}
const projection = (status, stage, sequence = 1) => ({
  version: 1,
  status,
  stage,
  mode: 'fixture-inspection',
  promptVersion: 'fixture-prompt-v1',
  schemaVersion: 'fixture-schema-v1',
  callId: status === 'INSPECTION_STATUS_CAPTURED' ? `actual-fixture-call-${sequence}` : undefined,
  issuedAt: status === 'INSPECTION_STATUS_CAPTURED' ? '2026-10-08T00:00:00Z' : undefined,
  fragments: [
    {
      id: 'system',
      role: 'INSPECTION_ROLE_SYSTEM',
      authorship: 'FRAGMENT_AUTHORSHIP_CODE',
      materialRole: 'instructions',
      text: '코드의 안전한 지시 😀\n두 번째 지시 줄',
      sourceFiles: ['composer.go'],
      activation: '해당 작업 단계',
    },
    {
      id: 'owner',
      role: 'INSPECTION_ROLE_USER',
      authorship: 'FRAGMENT_AUTHORSHIP_ACCOUNT',
      materialRole: 'owner-material',
      text: '소유자가 제공한 실제 자료',
      sourceRefs: ['memo'],
    },
  ],
  selectedRuleIds: ['safe-rule'],
  omissions: [
    {
      id: 'other-kind',
      reason: '선택한 작업에 적용되지 않습니다.',
      activation: '다른 종류',
      sourceFiles: ['other.go'],
    },
  ],
  output: { name: 'post', version: '1', schema: '<write>SAFE-GRAMMAR</write>' },
  conditions: {
    model: { providerId: 'fixture-provider', modelId: 'fixture-model' },
    maxCompletionTokens: '1234',
    reasoningEffort: 'low',
    structuredOutput: true,
    disableReasoning: false,
  },
  measures: {
    characters: '321',
    utf8Bytes: '777',
    referenceTokenEstimate: '90',
    ...(status === 'INSPECTION_STATUS_CAPTURED'
      ? { providerPromptTokens: '101', providerCompletionTokens: '202' }
      : {}),
  },
})
try {
  for (const theme of ['light', 'dark']) {
    for (const display of [
      { width: 320, height: 740, touch: true },
      { width: 390, height: 844, touch: true },
      { width: 1440, height: 900 },
      { width: 1920, height: 1080 },
      { width: 720, height: 450, zoom: 2 },
    ]) {
      const context = await browser.newContext({
        viewport: display,
        hasTouch: Boolean(display.touch),
        isMobile: Boolean(display.touch),
        deviceScaleFactor: display.zoom ?? 1,
        locale: 'ko-KR',
      })
      await context.addInitScript(
        ({ theme, ownerId }) => {
          localStorage.setItem('postpilot.theme', theme)
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
          window.__technicalCopies = []
          Object.defineProperty(navigator, 'clipboard', {
            value: { writeText: async (value) => window.__technicalCopies.push(value) },
          })
        },
        { theme, ownerId },
      )
      const page = await context.newPage()
      const calls = [],
        errors = []
      page.on('pageerror', (error) => errors.push(error.message))
      await page.route('**/postpilot.v1.*/**', async (route) => {
        const method = new URL(route.request().url()).pathname.split('/').at(-1)
        calls.push(method)
        let response = {}
        if (method === 'GetMe')
          response = {
            user: { id: ownerId, emailVerified: true, hasPassword: true },
            plan: 'PLAN_FREE',
          }
        if (method === 'GetMyPlan')
          response = { plan: 'PLAN_FREE', balance: { credits: 0, lots: [] } }
        if (method === 'GetPost') response = { post }
        if (method === 'GetPostRequestInspection') {
          const request = route.request().postDataJSON()
          const inspection =
            request.stage === 'revise'
              ? {
                  version: 1,
                  status: 'INSPECTION_STATUS_UNAVAILABLE',
                  stage: request.stage,
                  unavailableReason: 'capture_missing_stale_or_purged',
                }
              : projection(request.status, request.stage)
          response =
            request.status === 'INSPECTION_STATUS_CAPTURED' && request.stage !== 'revise'
              ? {
                  inspection: projection(request.status, request.stage, 2),
                  inspections: [inspection, projection(request.status, request.stage, 2)],
                }
              : { inspection }
        }
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify(response) })
      })
      await page.goto(`${url}/posts/${post.slug}`, { waitUntil: 'commit' })
      const opener = page.getByRole('button', { name: '요청 기술 보기', exact: true })
      await opener.waitFor()
      await page.waitForLoadState('networkidle')
      const workCalls = () =>
        calls.filter((name) =>
          /^(Start|Estimate|Save|Create|Patch|SetDefault|Initialize|Cancel|Finalize|Apply|Decide)/.test(
            name,
          ),
        )
      const existingHostWork = [...workCalls()]
      assert.equal(calls.includes('GetPostRequestInspection'), false)
      assert.equal(await page.getByText('SAFE-GRAMMAR', { exact: false }).count(), 0)
      await opener.focus()
      await page.keyboard.press('Enter')
      const dialog = page.getByRole('dialog', { name: '요청 기술 보기' })
      await dialog.waitFor()
      await dialog.getByText('소유자가 제공한 실제 자료', { exact: true }).waitFor()
      assert.ok((await dialog.innerText()).includes('현재 설정과 자료'))
      assert.equal(await dialog.getByText('actual-fixture-call-1', { exact: true }).count(), 0)
      for (const status of ['준비 미리보기', '실제 전송 기록']) {
        await dialog.getByRole('combobox', { name: /^요청 상태/ }).click()
        await page.getByRole('option', { name: status, exact: true }).click()
        await dialog.getByRole('heading', { name: status, exact: true }).first().waitFor()
      }
      await dialog.getByText('actual-fixture-call-2', { exact: true }).waitFor()
      assert.equal(await dialog.getByText('actual-fixture-call-1', { exact: true }).count(), 1)
      await dialog.getByRole('button', { name: '기술 요청 복사', exact: true }).click()
      await dialog.getByText('기술 요청을 복사했어요.', { exact: true }).waitFor()
      const copied = await page.evaluate(() => window.__technicalCopies.at(-1))
      assert.ok(
        copied.includes('actual-fixture-call-1') &&
          copied.includes('actual-fixture-call-2') &&
          copied.includes('<write>SAFE-GRAMMAR</write>'),
      )
      assert.equal(copied.includes('정식 본문'), false)
      const geometry = await dialog.evaluate((node) => {
        const elements = [...node.querySelectorAll('p,dt,dd,pre')]
        const minFont = Math.min(
          ...elements.map((entry) => Number.parseFloat(getComputedStyle(entry).fontSize)),
        )
        const horizontalOverflow =
          node.scrollWidth > node.clientWidth + 1 ||
          document.documentElement.scrollWidth > innerWidth + 1
        const scrollers = [...node.querySelectorAll('*')].filter(
          (entry) =>
            ['auto', 'scroll'].includes(getComputedStyle(entry).overflowY) &&
            entry.scrollHeight > entry.clientHeight + 1,
        ).length
        const canvas = document.createElement('canvas')
        canvas.width = canvas.height = 1
        const context = canvas.getContext('2d')
        const color = (value) => {
          context.clearRect(0, 0, 1, 1)
          context.fillStyle = value
          context.fillRect(0, 0, 1, 1)
          return [...context.getImageData(0, 0, 1, 1).data]
        }
        const luminance = (value) =>
          value
            .slice(0, 3)
            .map((entry) => entry / 255)
            .map((entry) => (entry <= 0.04045 ? entry / 12.92 : ((entry + 0.055) / 1.055) ** 2.4))
            .reduce((sum, entry, index) => sum + entry * [0.2126, 0.7152, 0.0722][index], 0)
        const ratios = elements
          .filter((entry) => entry.textContent.trim())
          .map((entry) => {
            let background = entry
            while (
              background.parentElement &&
              color(getComputedStyle(background).backgroundColor)[3] < 255
            )
              background = background.parentElement
            const foreground = luminance(color(getComputedStyle(entry).color)),
              surface = luminance(color(getComputedStyle(background).backgroundColor))
            return (Math.max(foreground, surface) + 0.05) / (Math.min(foreground, surface) + 0.05)
          })
        return {
          minFont,
          horizontalOverflow,
          scrollers,
          minContrast: Math.min(...ratios),
          closeHeight: node.querySelector('button')?.getBoundingClientRect().height,
        }
      })
      assert.ok(geometry.minFont >= 16)
      assert.equal(geometry.horizontalOverflow, false)
      assert.ok(geometry.scrollers <= 1)
      assert.ok(geometry.minContrast >= 4.5)
      assert.ok(geometry.closeHeight >= (display.touch ? 44 : 40))
      const screenshot = `${output}/${theme}-${display.width}${display.zoom ? '-zoom200' : ''}.png`
      await page.screenshot({ path: screenshot })
      await dialog.getByRole('combobox', { name: /^단계/ }).click()
      await page.getByRole('option', { name: '글 수정', exact: true }).click()
      await dialog
        .getByText('이 결과에 맞는 전송 기록이 없거나 삭제되었습니다.', { exact: true })
        .waitFor()
      assert.equal(
        await dialog.getByRole('button', { name: '기술 요청 복사', exact: true }).count(),
        0,
      )
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      assert.equal(await opener.evaluate((node) => node === document.activeElement), true)
      await opener.click()
      await dialog.getByText('소유자가 제공한 실제 자료', { exact: true }).waitFor()
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      assert.equal(calls.filter((name) => name === 'GetPostRequestInspection').length, 5)
      assert.deepEqual(workCalls(), existingHostWork)
      assert.deepEqual(errors, [])
      results.push({
        theme,
        width: display.width,
        zoom: display.zoom ?? 1,
        ...geometry,
        reads: 5,
        noAdditionalWorkCalls: true,
        existingHostWork,
        screenshot,
      })
      await context.close()
    }
  }
  const paths = [
    'frontend/src/pages/editor/ui/DraftEditor.tsx',
    'frontend/src/pages/editor/ui/LifecycleSteps.tsx',
    'frontend/src/widgets/ai-authoring-studio/ui/AIAuthoringStudio.tsx',
    'frontend/src/features/ai-authoring/ui/AuthoringEditor.tsx',
    'frontend/src/widgets/writing-test/ui/WritingTestPair.tsx',
    'frontend/src/entities/request-inspection/api/hooks.ts',
    'frontend/src/entities/request-inspection/api/client.ts',
    'frontend/src/features/inspect-writing-request/ui/InspectWritingRequestAction.tsx',
    'frontend/src/features/inspect-writing-request/ui/RequestInspectionDocument.tsx',
    'frontend/src/features/inspect-writing-request/model/request-document.ts',
    'frontend/src/pages/editor/lib/request-inspection.browser.mjs',
  ]
  const sourceFiles = Object.fromEntries(
    await Promise.all(
      paths.map(async (path) => [
        path,
        createHash('sha256')
          .update(await readFile(path))
          .digest('hex'),
      ]),
    ),
  )
  await writeFile(
    `${output}/measurements.json`,
    JSON.stringify(
      {
        fixture: 'Synthetic intercepted owner RPCs; no backend or provider calls',
        zoomMethod:
          '200% desktop reflow emulation at720x450 CSS pixels and DPR2, not native browser menu zoom',
        sourceFiles,
        results,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(JSON.stringify({ cases: results.length, output }))
} finally {
  await browser.close()
}
