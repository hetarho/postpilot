import { chromium } from 'playwright'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import assert from 'node:assert/strict'

const output = process.env.T646_BROWSER_OUTPUT ?? '/tmp/postpilot-t646-browser'
const url = process.env.T646_WEB_URL ?? 'http://127.0.0.1:26143'
await mkdir(output, { recursive: true })
// This literal deliberately resembles review markup but is genuine canonical owner wording.
// Keep it; label/class based stripping would erase the author's words.
const OWNER_WORDS =
  '직접 입력 기반 · 사진에서 추론 · AI가 보탠 내용 · 출처 미확인 · 요청 기술 보기 · <write>원문 & 😀</write> · 사진_7_내_원문_사진 · <span class="text-origin-owner-foreground">출처 보기</span>'
const OWNER_ALT_LABEL = '소유자 원문: 대체 텍스트 출처.\n둘째 줄도 직접 썼습니다.'
const PRIVATE_ORIGIN = 'PRIVATE-ORIGIN-SOURCE-EXPLANATION'
const PRIVATE_REQUEST = 'PRIVATE-TECHNICAL-REQUEST-METADATA'
const finalParagraph = `최종 직접 입력 😀로 고친 문장입니다. ${OWNER_WORDS} ${OWNER_ALT_LABEL}`
const content = {
  title: '서울 😀에서 찾은 카페',
  summary: '직접 방문한 기록과 사진 속 풍경',
  tags: ['서울카페', '커피'],
  blocks: [
    {
      type: 'TEXT',
      content: `서울 😀에서 커피 사진을 보고 행복을 상상했어요. 서울 😀의 기록입니다. ${OWNER_WORDS} ${OWNER_ALT_LABEL}`,
    },
    { type: 'HEADING', content: '사진으로 보는 커피', level: 2 },
    { type: 'QUOTE', content: '이전 원본이 없는 문구' },
    { type: 'LIST', items: ['직접 방문한 서울 😀', '사진에 보이는 커피'] },
    { type: 'IMAGE', file: 'coffee.jpg', alt: '창가의 커피 사진', caption: '커피가 놓인 창가' },
    {
      type: 'GALLERY',
      files: ['coffee.jpg', 'window.jpg'],
      layout: 'GALLERY_LAYOUT_COLLAGE',
      alt: '두 장의 카페 사진',
      caption: '커피와 창가 사진 묶음',
    },
    { type: 'VIDEO', file: 'cafe.mp4', alt: '카페 창가 영상', caption: '카페 영상의 창가' },
  ],
}
const sources = [
  {
    id: 'memo',
    kind: 'memo',
    text: `${PRIVATE_ORIGIN}: 직접 서울 카페에 방문했어요.`,
    available: true,
  },
  {
    id: 'visual',
    kind: 'visual_observation',
    text: '창가 테이블 위에 커피잔이 보입니다.',
    attachmentFilename: 'coffee.jpg',
    attachmentId: 'photo-a',
    available: true,
  },
  {
    id: 'proposal',
    kind: 'ai_proposal',
    text: '행복은 입력이나 사진으로 확인되지 않은 AI 제안입니다.',
    available: true,
  },
]
sources.push({
  id: 'withdrawn',
  kind: 'visual_observation',
  text: 'WITHDRAWN-SOURCE-MUST-NOT-BE-RETARGETED',
  attachmentFilename: 'withdrawn.jpg',
  attachmentId: 'deleted-photo',
  available: true,
})
const categories = ['OWNER_INPUT', 'PHOTO_INTERPRETATION', 'AI_ADDED']
const refs = ['memo', 'visual', 'proposal']
const spans = []
function span(field, text, category = 0, quote = text, offset = 0) {
  const start = Array.from(text.slice(0, offset)).length
  spans.push({
    field: { ...field, kind: 'ORIGIN_FIELD_KIND_' + field.kind },
    start,
    end: start + Array.from(quote).length,
    quote,
    category: 'SEMANTIC_ORIGIN_CATEGORY_' + categories[category],
    sourceRefs: [refs[category]],
    reviewState: 'ORIGIN_REVIEW_STATE_UNREVIEWED',
  })
}
span({ kind: 'TITLE' }, content.title)
span({ kind: 'SUMMARY' }, content.summary, 0)
content.tags.forEach((text, tagIndex) => span({ kind: 'TAG', tagIndex }, text, 0))
content.blocks.forEach((block, blockIndex) => {
  if (blockIndex === 0) {
    for (const [quote, category] of [
      ['서울 😀', 0],
      ['커피 사진', 1],
      ['행복을 상상했어요', 2],
    ])
      span(
        { kind: 'BLOCK_CONTENT', blockIndex },
        block.content,
        category,
        quote,
        block.content.indexOf(quote),
      )
    span(
      { kind: 'BLOCK_CONTENT', blockIndex },
      block.content,
      0,
      '서울 😀',
      block.content.lastIndexOf('서울 😀'),
    )
  } else if (block.content)
    span({ kind: 'BLOCK_CONTENT', blockIndex }, block.content, blockIndex % 3)
  block.items?.forEach((text, itemIndex) =>
    span({ kind: 'BLOCK_ITEM', blockIndex, itemIndex }, text, itemIndex),
  )
  if (block.alt) span({ kind: 'BLOCK_ALT', blockIndex }, block.alt, 1)
  if (block.caption) span({ kind: 'BLOCK_CAPTION', blockIndex }, block.caption, 1)
})
async function readNativeCopy(page, locator, keepSelection = false) {
  if (!keepSelection)
    await locator.evaluate((node) => {
      const range = document.createRange()
      range.selectNodeContents(node)
      const selection = getSelection()
      selection.removeAllRanges()
      selection.addRange(range)
    })
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+c' : 'Control+c')
  return page.evaluate(async () => {
    const items = await navigator.clipboard.read()
    const copied = {}
    for (const item of items)
      for (const type of item.types)
        if (type === 'text/plain' || type === 'text/html')
          copied[type] = await (await item.getType(type)).text()
    return copied
  })
}
const unavailable = (stage, reason = 'capture_missing_stale_or_purged') => ({
  version: 1,
  status: 'INSPECTION_STATUS_UNAVAILABLE',
  stage,
  unavailableReason: reason,
})
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
      text: 'PRIVATE-TECHNICAL-REQUEST-METADATA: 소유자가 제공한 실제 자료',
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
spans.find((entry) => entry.field.blockIndex === 2).sourceRefs = ['withdrawn']
spans.find((entry) => entry.field.blockIndex === 2).category =
  'SEMANTIC_ORIGIN_CATEGORY_PHOTO_INTERPRETATION'

async function reachable(locator) {
  await locator.evaluate((node) => node.scrollIntoView({ block: 'center' }))
  return locator.evaluate((node) => {
    // Inline spans can wrap: their union bounding box contains whitespace that is not a target.
    // Hit real line-fragment rectangles instead of claiming that whitespace is an obstruction.
    const points = [...node.getClientRects()].map((rect) => {
      const x = rect.x + rect.width / 2,
        y = rect.y + rect.height / 2
      const hit = document.elementFromPoint(x, y)
      return {
        x,
        y,
        width: rect.width,
        height: rect.height,
        reachable: hit === node || node.contains(hit),
        hitTag: hit?.tagName,
      }
    })
    window.__hitEvidence ??= []
    window.__hitEvidence.push({
      name: node.getAttribute('aria-label') || node.textContent || node.type,
      points,
    })
    return points.some((point) => point.reachable)
  })
}
async function cleanNativeCopy(page, locator, expected) {
  const copied = await readNativeCopy(page, locator)
  assert.equal(copied['text/plain'], expected)
  const html = copied['text/html'] ?? ''
  const projection = await page.evaluate((html) => {
    const fragment = new DOMParser().parseFromString(html, 'text/html')
    return {
      text: fragment.body.textContent,
      annotated: fragment.querySelectorAll('[style], [role], [aria-label]').length,
    }
  }, html)
  assert.equal(projection.text, expected)
  assert.equal(
    projection.annotated,
    0,
    'Native HTML must exclude review highlight styles and controls',
  )
  assert.equal(html.includes(PRIVATE_ORIGIN), false)
  assert.equal(html.includes(PRIVATE_REQUEST), false)
  if (expected.includes('text-origin-owner-foreground')) {
    assert.equal(html.split('text-origin-owner-foreground').length - 1, 1)
    assert.ok(html.includes('&lt;span'), 'Owner literal markup must remain escaped canonical text')
  }
  await page.evaluate(() => getSelection().removeAllRanges())
  return copied
}
async function inspectTechnical(page, screenshot) {
  const opener = page.getByRole('button', { name: '요청 기술 보기', exact: true })
  await opener.focus()
  await page.keyboard.press('Enter')
  const dialog = page.getByRole('dialog', { name: '요청 기술 보기' })
  await dialog.getByText(`${PRIVATE_REQUEST}: 소유자가 제공한 실제 자료`, { exact: true }).waitFor()
  assert.ok((await dialog.innerText()).includes('실제 전송한 요청이 아닙니다.'))
  for (const status of ['준비 미리보기', '실제 전송 기록']) {
    await dialog.getByRole('combobox', { name: /^요청 상태/ }).click()
    await page.getByRole('option', { name: status, exact: true }).click()
    await dialog.getByRole('heading', { name: status, exact: true }).first().waitFor()
  }
  await dialog.getByText('actual-fixture-call-2', { exact: true }).waitFor()
  await dialog.getByRole('button', { name: '기술 요청 복사', exact: true }).click()
  await dialog.getByText('기술 요청을 복사했어요.', { exact: true }).waitFor()
  const copied = await page.evaluate(() => window.__explicitCopies.at(-1))
  assert.ok(copied.includes('actual-fixture-call-1') && copied.includes('actual-fixture-call-2'))
  assert.ok(copied.includes(PRIVATE_REQUEST))
  assert.equal(copied.includes(OWNER_WORDS), false)
  const geometry = await documentGeometry(dialog)
  assert.ok(geometry.minFont >= 16)
  assert.ok(geometry.minContrast >= 4.5)
  assert.equal(geometry.horizontalOverflow, false)
  assert.ok(geometry.scrollers <= 1)
  assert.equal(geometry.closeReachable, true)
  await page.screenshot({ animations: 'disabled', path: screenshot })
  await dialog.getByRole('combobox', { name: /^단계/ }).click()
  await page.getByRole('option', { name: '글 수정', exact: true }).click()
  await dialog
    .getByText('이 결과에 맞는 전송 기록이 없거나 삭제되었습니다.', { exact: true })
    .waitFor()
  assert.equal(await dialog.getByRole('button', { name: '기술 요청 복사', exact: true }).count(), 0)
  assert.equal((await dialog.innerText()).includes(PRIVATE_REQUEST), false)
  await page.keyboard.press('Escape')
  await dialog.waitFor({ state: 'hidden' })
  assert.equal(await opener.evaluate((node) => node === document.activeElement), true)
  return { geometry, copied, missingHistorySafe: true, screenshot }
}
async function documentGeometry(locator) {
  return locator.evaluate((node) => {
    const elements = [...node.querySelectorAll('p,dt,dd,pre')].filter((entry) =>
      entry.textContent.trim(),
    )
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
        .map((n) => n / 255)
        .map((n) => (n <= 0.04045 ? n / 12.92 : ((n + 0.055) / 1.055) ** 2.4))
        .reduce((sum, n, i) => sum + n * [0.2126, 0.7152, 0.0722][i], 0)
    const contrasts = elements.map((entry) => {
      let surface = entry
      while (surface.parentElement && color(getComputedStyle(surface).backgroundColor)[3] < 255)
        surface = surface.parentElement
      const fg = luminance(color(getComputedStyle(entry).color)),
        bg = luminance(color(getComputedStyle(surface).backgroundColor))
      return (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05)
    })
    return {
      minFont: Math.min(
        ...elements.map((entry) => Number.parseFloat(getComputedStyle(entry).fontSize)),
      ),
      minContrast: Math.min(...contrasts),
      horizontalOverflow:
        node.scrollWidth > node.clientWidth + 1 ||
        document.documentElement.scrollWidth > innerWidth + 1,
      scrollers: [...node.querySelectorAll('*')].filter(
        (entry) =>
          ['auto', 'scroll'].includes(getComputedStyle(entry).overflowY) &&
          entry.scrollHeight > entry.clientHeight + 1,
      ).length,
      closeHeight: node.querySelector('button')?.getBoundingClientRect().height,
      closeReachable: (() => {
        const close = node.querySelector('button')
        const rect = close.getBoundingClientRect()
        const hit = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2)
        return hit === close || close.contains(hit)
      })(),
    }
  })
}
async function exportCanonical(page, expectedParagraph) {
  const panel = page.getByRole('region', { name: '내보내기', exact: true })
  await panel.waitFor()
  const values = []
  const callsBefore = await page.evaluate(() => window.__rpcCount)
  for (const label of ['네이버 블로그', '티스토리', '자체 사이트', '마크다운']) {
    await panel.getByRole('tab', { name: label, exact: true }).click()
    await panel.getByRole('button', { name: '복사', exact: true }).click()
    const value = await page.evaluate(() => window.__explicitCopies.at(-1))
    assert.ok(
      value.includes(
        label === '네이버 블로그'
          ? expectedParagraph
          : expectedParagraph
              .replaceAll('&', '&amp;')
              .replaceAll('<', '&lt;')
              .replaceAll('>', '&gt;')
              .replaceAll('"', '&quot;')
              .replaceAll("'", '&#39;'),
      ),
    )
    assert.equal(value.includes(PRIVATE_ORIGIN), false)
    assert.equal(value.includes(PRIVATE_REQUEST), false)
    assert.equal(value.includes('WITHDRAWN-SOURCE-MUST-NOT-BE-RETARGETED'), false)
    assert.equal(value.split('text-origin-owner-foreground').length - 1, 1)
    await page.evaluate(() => {
      window.__forceCopyFailure = true
    })
    await panel.getByRole('button', { name: '복사', exact: true }).click()
    const fallback = panel.getByRole('textbox', { name: '내보내기 결과', exact: true })
    await fallback.waitFor()
    const selected = await fallback.evaluate((node) => ({
      value: node.value,
      start: node.selectionStart,
      end: node.selectionEnd,
      focused: node === document.activeElement,
    }))
    assert.equal(selected.value, value)
    assert.equal(selected.start, 0)
    assert.equal(selected.end, value.length)
    assert.equal(selected.focused, true)
    await page.evaluate(() => {
      window.__forceCopyFailure = false
    })
    values.push({ format: label, value, manualSelection: selected })
  }
  assert.equal(await page.evaluate(() => window.__rpcCount), callsBefore)
  return values
}
async function inspectBlind(browser, theme, width) {
  const context = await browser.newContext({
    viewport: { width, height: width < 640 ? 800 : 900 },
    hasTouch: width < 640,
    isMobile: width < 640,
    locale: 'ko-KR',
  })
  await context.addInitScript((theme) => {
    localStorage.setItem('postpilot.theme', theme)
    localStorage.setItem('postpilot.locale', 'ko')
    localStorage.setItem(
      'postpilot.setup.v1.t646-browser',
      JSON.stringify({
        version: 1,
        ownerId: 't646-browser',
        completed: true,
        skipped: [],
        resume: 'welcome',
        target: '/',
      }),
    )
    window.__explicitCopies = []
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: async (value) => window.__explicitCopies.push(value) },
    })
  }, theme)
  const page = await context.newPage()
  const calls = [],
    errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  const test = {
    id: 't646-blind',
    revision: 4,
    factor: 'WRITING_TEST_FACTOR_MODEL',
    modelStage: 'WRITING_TEST_STAGE_WRITE',
    count: 2,
    status: 'WRITING_TEST_STATUS_REVIEW',
    revealed: false,
    createdAt: '2026-10-08T00:00:00Z',
    updatedAt: '2026-10-08T00:00:01Z',
    contentExpiresAt: '2099-01-01T00:00:00Z',
    targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
    candidates: [0, 1].map((index) => ({
      id: `candidate-${index}`,
      displayLabel: String.fromCharCode(65 + index),
      status: 'WRITING_TEST_CANDIDATE_STATUS_SUCCEEDED',
      output: {
        title: `후보 ${index + 1}의 소유자 글`,
        blocks: [{ type: 'TEXT', content: OWNER_WORDS }],
      },
    })),
    matches: [
      {
        id: 'match-1-0',
        round: 1,
        index: 0,
        leftCandidateId: 'candidate-0',
        rightCandidateId: 'candidate-1',
      },
    ],
  }
  await page.route('**/postpilot.v1.*/**', async (route) => {
    const method = new URL(route.request().url()).pathname.split('/').at(-1)
    calls.push(method)
    let response = {}
    if (method === 'GetMe')
      response = {
        user: { id: 't646-browser', emailVerified: true, hasPassword: true },
        plan: 'PLAN_FREE',
      }
    if (method === 'GetMyPlan') response = { plan: 'PLAN_FREE', balance: { credits: 0, lots: [] } }
    if (method === 'GetWritingTest') response = { test }
    if (method === 'GetWritingTestRequestInspection') {
      const request = route.request().postDataJSON()
      response = {
        inspection: {
          ...unavailable(request.stage, 'blind_test_identity_hidden_until_reveal'),
          fragments: [
            {
              id: 'blind-identity',
              role: 'INSPECTION_ROLE_SYSTEM',
              authorship: 'FRAGMENT_AUTHORSHIP_CODE',
              text: 'BLIND-IDENTITY-SDK-COST-MUST-NOT-LEAK',
            },
          ],
          conditions: { model: { providerId: 'BLIND-PROVIDER', modelId: 'BLIND-MODEL' } },
          sdkUrl: 'https://identity.example.test/sdk',
          supplierCost: 999,
        },
      }
    }
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify(response) })
  })
  await page.goto(`${url}/tests/${test.id}`, { waitUntil: 'commit' })
  const opener = page.getByRole('button', { name: '요청 기술 보기', exact: true }).first()
  await opener.waitFor()
  await page.waitForLoadState('networkidle')
  const work = () =>
    calls.filter((name) =>
      /^(Start|Estimate|Save|Create|Patch|SetDefault|Initialize|Cancel|Finalize|Apply|Decide)/.test(
        name,
      ),
    )
  const before = [...work()]
  assert.equal(calls.includes('GetWritingTestRequestInspection'), false)
  await opener.focus()
  await page.keyboard.press('Enter')
  const dialog = page.getByRole('dialog', { name: '요청 기술 보기' })
  await dialog
    .getByText(
      '블라인드 비교 중에는 참가자의 정체가 드러나는 요청을 공개하지 않습니다. 기존 결과 공개 후 확인할 수 있습니다.',
      { exact: true },
    )
    .waitFor()
  for (const label of ['현재 설정', '준비 미리보기']) {
    await dialog.getByRole('combobox', { name: /^요청 상태/ }).click()
    await page.getByRole('option', { name: label, exact: true }).click()
    await dialog
      .getByText(
        '블라인드 비교 중에는 참가자의 정체가 드러나는 요청을 공개하지 않습니다. 기존 결과 공개 후 확인할 수 있습니다.',
        { exact: true },
      )
      .waitFor()
  }
  assert.equal(await dialog.getByRole('button', { name: '기술 요청 복사', exact: true }).count(), 0)
  assert.equal(
    /BLIND-(IDENTITY|PROVIDER|MODEL)|identity\.example|999/.test(
      await page.locator('body').innerText(),
    ),
    false,
  )
  assert.deepEqual(await page.evaluate(() => window.__explicitCopies), [])
  const geometry = await documentGeometry(dialog)
  assert.equal(geometry.horizontalOverflow, false)
  assert.ok(geometry.minFont >= 16 && geometry.minContrast >= 4.5 && geometry.scrollers <= 1)
  assert.ok(geometry.closeHeight >= (width < 640 ? 44 : 40))
  const screenshot = `${output}/${theme}-${width}-blind.png`
  await page.screenshot({ animations: 'disabled', path: screenshot })
  await page.keyboard.press('Escape')
  await dialog.waitFor({ state: 'hidden' })
  assert.equal(await opener.evaluate((node) => node === document.activeElement), true)
  assert.deepEqual(work(), before)
  assert.deepEqual(errors, [])
  await context.close()
  return {
    theme,
    width,
    authoritativeSyntheticBlindDenial: true,
    entirePrivateProjectionRemoved: true,
    geometry,
    calls,
    screenshot,
    noAdditionalWorkCalls: true,
  }
}
const productionPaths = [
  'frontend/dist/index.html',
  ...execFileSync('rg', ['--files', 'frontend/dist/assets'], { encoding: 'utf8' })
    .trim()
    .split('\n')
    .filter((path) => /\.(js|css)$/.test(path)),
]
const productSourcePaths = execFileSync(
  'git',
  ['ls-files', '--cached', '--others', '--exclude-standard', '--', 'frontend/src'],
  { encoding: 'utf8' },
)
  .trim()
  .split('\n')
  .filter((path) => path && !path.includes('.test.') && !path.includes('__snapshots__'))
const hashFiles = async (paths) =>
  Object.fromEntries(
    await Promise.all(
      paths.map(async (path) => [
        path,
        createHash('sha256')
          .update(await readFile(path))
          .digest('hex'),
      ]),
    ),
  )
const productionFingerprintBefore = await hashFiles(productionPaths)
const sourceFingerprintBefore = await hashFiles(productSourcePaths)
const browser = await chromium.launch({ headless: true })
const results = []
const blindResults = []
try {
  for (const theme of process.env.T646_BROWSER_SMOKE ? ['light'] : ['light', 'dark']) {
    for (const display of [
      { width: 320, height: 640, touch: true },
      { width: 360, height: 800, touch: true },
      { width: 390, height: 844, touch: true },
      { width: 430, height: 932, touch: true },
      { width: 1440, height: 900 },
      { width: 1920, height: 1080 },
      { width: 720, height: 450, zoom: 2 },
    ]) {
      if (process.env.T646_BROWSER_SMOKE && display.width !== 390) continue
      const context = await browser.newContext({
        permissions: ['clipboard-read', 'clipboard-write'],
        viewport: display,
        hasTouch: Boolean(display.touch),
        isMobile: Boolean(display.touch),
        deviceScaleFactor: display.zoom ?? 1,
        locale: 'ko-KR',
      })
      await context.addInitScript((theme) => {
        localStorage.setItem('postpilot.theme', theme)
        localStorage.setItem('postpilot.locale', 'ko')
        window.__explicitCopies = []
        window.__rpcCount = 0
        const nativeClipboard = navigator.clipboard
        Object.defineProperty(navigator, 'clipboard', {
          value: {
            read: () => nativeClipboard.read(),
            writeText: async (value) => {
              if (window.__forceCopyFailure) throw new Error('Synthetic clipboard refusal')
              window.__explicitCopies.push(value)
              await nativeClipboard.writeText(value)
            },
          },
        })
        const nativeExec = document.execCommand.bind(document)
        document.execCommand = (command, ...args) =>
          window.__forceCopyFailure && command === 'copy' ? false : nativeExec(command, ...args)
        localStorage.setItem(
          'postpilot.setup.v1.t646-browser',
          JSON.stringify({
            version: 1,
            ownerId: 't646-browser',
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
      const saveRequests = [],
        finalizeRequests = []
      let retainMissingOrigins = false
      page.on('pageerror', (error) => errors.push(error.message))
      let post = {
        slug: 't646-origins',
        createdAt: '2026-10-08T00:00:00Z',
        inputRevision: '3',
        status: 'review',
        title: '서울 카페 기록',
        memo: '직접 방문한 기록',
        content,
        contentRevision: '1',
        machineBaselineRevision: '1',
        contentHash: 'fixture-1',
        targetLanguage: 'CONTENT_LANGUAGE_KOREAN',
        contentLanguage: 'CONTENT_LANGUAGE_KOREAN',
        canFinalize: true,
        contentOrigins: {
          version: 1,
          result: { contentRevision: '1', contentHash: 'fixture-1' },
          sources,
          spans,
        },
        images: [
          {
            id: 'photo-a',
            filename: 'coffee.jpg',
            width: 640,
            height: 320,
            viewUrl: '/t646-photo.svg',
          },
          {
            id: 'photo-b',
            filename: 'window.jpg',
            width: 640,
            height: 320,
            viewUrl: '/t646-photo.svg',
          },
        ],
        videos: [{ id: 'video-a', filename: 'cafe.mp4' }],
      }
      await page.route('**/t646-photo.svg', (route) =>
        route.fulfill({
          contentType: 'image/svg+xml',
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="320"><rect width="640" height="320" fill="#ddd"/><text x="30" y="170" font-size="30">Fixture photo</text></svg>',
        }),
      )
      await page.route('**/postpilot.v1.*/**', async (route) => {
        const requestPath = new URL(route.request().url()).pathname
        if (!requestPath.includes('/postpilot.v1.')) return route.continue()
        const method = requestPath.split('/').at(-1)
        calls.push(method)
        await page.evaluate(() => {
          window.__rpcCount += 1
        })
        let response = {}
        if (method === 'GetMe')
          response = {
            user: {
              id: 't646-browser',
              email: 'fixture@example.test',
              emailVerified: true,
              hasPassword: true,
            },
            plan: 'PLAN_FREE',
          }
        if (method === 'GetMyPlan')
          response = { plan: 'PLAN_FREE', balance: { credits: 0, lots: [] } }
        if (method === 'GetPost') response = { post }
        if (method === 'GetPostRequestInspection') {
          const request = route.request().postDataJSON()
          const inspection =
            request.stage === 'revise'
              ? unavailable(request.stage)
              : projection(request.status, request.stage)
          response =
            request.status === 'INSPECTION_STATUS_CAPTURED' && request.stage !== 'revise'
              ? {
                  inspection: projection(request.status, request.stage, 2),
                  inspections: [inspection, projection(request.status, request.stage, 2)],
                }
              : { inspection }
        }
        if (method === 'FinalizePost') {
          const request = route.request().postDataJSON()
          finalizeRequests.push(request)
          assert.equal(request.expectedRevision, post.contentRevision)
          post = {
            ...post,
            status: 'finalized',
            finalizedRevision: post.contentRevision,
            title: post.content.title,
          }
          response = { post }
        }
        if (method === 'SavePostContent') {
          const request = route.request().postDataJSON()
          saveRequests.push(request)
          post = {
            ...post,
            content: request.content,
            contentRevision: String(Number(post.contentRevision) + 1),
            contentHash: 'saved',
            contentOrigins: retainMissingOrigins
              ? undefined
              : {
                  version: 1,
                  result: {
                    contentRevision: String(Number(post.contentRevision) + 1),
                    contentHash: 'saved',
                  },
                  sources,
                  spans: [
                    {
                      ...spans[0],
                      quote: 'INVALID-ANNOTATION-MUST-NOT-HIDE-VALID-BODY',
                      end: 99999,
                    },
                  ],
                },
          }
          response = { post }
        }
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify(response) })
      })
      await page.goto(url + '/posts/t646-origins', { waitUntil: 'commit' })
      const article = page.getByRole('article', { name: '생성된 글' })
      try {
        await article.waitFor({ timeout: 15000 })
      } catch (error) {
        console.log(JSON.stringify({ body: await page.locator('body').innerText(), errors, calls }))
        throw error
      }
      await page.evaluate(() => document.fonts.ready)
      const pointer = await page.evaluate(() => ({
        coarse: matchMedia('(pointer:coarse)').matches,
        fine: matchMedia('(pointer:fine)').matches,
      }))
      assert.equal(pointer.coarse, Boolean(display.touch))
      assert.equal(pointer.fine, !display.touch)
      assert.equal(await page.getByRole('checkbox', { name: '출처 보기' }).isChecked(), true)
      assert.equal(
        await page.getByRole('list', { name: '의미의 출처' }).getByRole('listitem').count(),
        3,
      )
      assert.equal(await page.getByRole('group', { name: '대체 텍스트 출처' }).count(), 3)
      assert.equal(
        await page.getByRole('img', { name: '창가의 커피 사진', exact: true }).count(),
        1,
      )
      const toggleReachable = await reachable(
        page.getByRole('checkbox', { name: '출처 보기', exact: true }),
      )
      const finderReachable = await reachable(
        page.getByRole('button', { name: '문구 출처 찾기', exact: true }),
      )
      assert.equal(toggleReachable, true, 'Origin toggle must remain hittable after safe scrolling')
      assert.equal(finderReachable, true, 'Phrase finder must remain hittable after safe scrolling')
      const phrase = article
        .getByRole('button', { name: '직접 입력 기반 출처 보기: 서울 😀', exact: true })
        .first()
      await phrase.focus()
      await page.keyboard.press('Enter')
      const dialog = page.getByRole('dialog')
      await dialog.waitFor()
      assert.ok((await dialog.innerText()).includes('직접 서울 카페에 방문했어요.'))
      await dialog.evaluate(async (node) => {
        await Promise.all(node.getAnimations().map((animation) => animation.finished))
      })
      const sourceGeometry = await documentGeometry(dialog)
      assert.equal(sourceGeometry.closeReachable, true)
      assert.ok(sourceGeometry.minFont >= 16 && sourceGeometry.minContrast >= 4.5)
      assert.equal(sourceGeometry.horizontalOverflow, false)
      assert.ok(sourceGeometry.scrollers <= 1)
      const sourceScreenshot = `${output}/${theme}-${display.width}${display.zoom ? '-zoom200' : ''}-source.png`
      await page.screenshot({ animations: 'disabled', path: sourceScreenshot })
      await page.keyboard.press('Tab')
      assert.equal(await dialog.evaluate((node) => node.contains(document.activeElement)), true)
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      assert.equal(await phrase.evaluate((node) => node === document.activeElement), true)
      if (display.touch) {
        const entry = page.getByRole('button', { name: '문구 출처 찾기', exact: true })
        const target = await entry.boundingBox()
        assert.ok(target.width >= 44 && target.height >= 44)
        await entry.tap()
        await dialog.waitFor()
        const choice = dialog
          .getByRole('button', { name: '본문 1 · 직접 입력 기반 · 서울 😀', exact: true })
          .first()
        const choiceTarget = await choice.boundingBox()
        assert.ok(choiceTarget.width >= 44 && choiceTarget.height >= 44)
        await choice.tap()
        assert.ok((await dialog.innerText()).includes('직접 서울 카페에 방문했어요.'))
        await dialog.getByRole('button', { name: '닫기', exact: true }).tap()
        await dialog.waitFor({ state: 'hidden' })
        assert.equal(await entry.evaluate((node) => node === document.activeElement), true)
      }
      // A drag/copy selection is ordinary text, even when the phrase is clickable.
      const selected = await phrase.evaluate((node) => {
        const range = document.createRange()
        range.selectNodeContents(node)
        const selection = getSelection()
        selection.removeAllRanges()
        selection.addRange(range)
        return selection.toString()
      })
      await phrase.click()
      assert.equal(await dialog.count(), 0)
      assert.equal(await page.evaluate(() => getSelection().toString()), selected)
      const nativeClipboard = [await cleanNativeCopy(page, phrase, '서울 😀')]
      const firstParagraph = article.locator('p').filter({ hasText: OWNER_WORDS }).first()
      await page.evaluate(() => document.activeElement.blur())
      nativeClipboard.push(await cleanNativeCopy(page, firstParagraph, content.blocks[0].content))
      const wholeArticleCopy = await readNativeCopy(page, article)
      assert.ok(wholeArticleCopy['text/plain'].includes(content.blocks[0].content))
      assert.equal(
        wholeArticleCopy['text/plain'].split('대체 텍스트 출처').length - 1,
        1,
        'Exclude only review label nodes, preserving identical canonical owner wording',
      )
      const wholeHtml = wholeArticleCopy['text/html']
      assert.equal(wholeHtml.split('대체 텍스트 출처').length - 1, 1)
      assert.equal(/style=|aria-|role=|data-writing-origin/.test(wholeHtml), false)
      assert.equal(wholeHtml.includes('/t646-photo.svg'), false)
      assert.equal(wholeHtml.includes(PRIVATE_ORIGIN), false)
      assert.equal(wholeHtml.split('text-origin-owner-foreground').length - 1, 1)
      for (const canonicalText of [
        '창가의 커피 사진',
        '커피가 놓인 창가',
        '두 장의 카페 사진',
        '커피와 창가 사진 묶음',
        '카페 창가 영상',
        '카페 영상의 창가',
      ]) {
        assert.ok(wholeArticleCopy['text/plain'].includes(canonicalText))
        assert.ok(wholeHtml.includes(canonicalText))
      }
      nativeClipboard.push(wholeArticleCopy)
      await page.evaluate(() => getSelection().removeAllRanges())
      const partialSelection = await phrase.evaluate((node) => {
        const range = document.createRange()
        range.setStart(node.firstChild, 1)
        range.setEnd(node.firstChild, 5)
        getSelection().removeAllRanges()
        getSelection().addRange(range)
        return getSelection().toString()
      })
      assert.equal(partialSelection, '울 😀')
      const partialCopy = await readNativeCopy(page, phrase, true)
      assert.equal(partialCopy['text/plain'], partialSelection)
      assert.equal(/style=|aria-|role=/.test(partialCopy['text/html']), false)
      nativeClipboard.push(partialCopy)
      const multiSelection = await article.evaluate((node) => {
        const paragraph = [...node.querySelectorAll('p')].find((entry) =>
          entry.textContent.includes('사진_7_내_원문_사진'),
        )
        const heading = [...node.querySelectorAll('h2,h3,h4')].find(
          (entry) => entry.textContent === '사진으로 보는 커피',
        )
        const range = document.createRange()
        range.setStartBefore(paragraph)
        range.setEndAfter(heading)
        getSelection().removeAllRanges()
        getSelection().addRange(range)
        return getSelection().toString()
      })
      const multiCopy = await readNativeCopy(page, article, true)
      assert.equal(multiCopy['text/plain'], multiSelection)
      assert.ok(multiCopy['text/html'].includes('<p>') && /<h[234]>/.test(multiCopy['text/html']))
      assert.equal(/style=|aria-|role=/.test(multiCopy['text/html']), false)
      assert.ok(multiCopy['text/html'].includes('&lt;span'))
      nativeClipboard.push(multiCopy)
      await page.evaluate(() => getSelection().removeAllRanges())
      await page.evaluate(() => getSelection().removeAllRanges())
      const aiPhrase = article.getByRole('button', {
        name: 'AI가 보탠 내용 출처 보기: 행복을 상상했어요',
        exact: true,
      })
      assert.equal(await reachable(aiPhrase), true)
      await aiPhrase.focus()
      await page.keyboard.press(' ')
      await dialog.waitFor()
      assert.ok(
        (await dialog.innerText()).includes(
          '행복은 입력이나 사진으로 확인되지 않은 AI 제안입니다.',
        ),
      )
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      assert.equal(await aiPhrase.evaluate((node) => node === document.activeElement), true)
      const missingSource = article.getByRole('button', {
        name: '출처 미확인 출처 보기: 이전 원본이 없는 문구',
        exact: true,
      })
      await missingSource.focus()
      await page.keyboard.press(' ')
      await dialog.waitFor()
      assert.ok((await dialog.innerText()).includes('원본이 삭제되었거나 이용할 수 없습니다.'))
      assert.equal(
        (await dialog.innerText()).includes('WITHDRAWN-SOURCE-MUST-NOT-BE-RETARGETED'),
        false,
      )
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      const technical = await inspectTechnical(
        page,
        `${output}/${theme}-${display.width}${display.zoom ? '-zoom200' : ''}-technical.png`,
      )
      const reachedSources = []
      for (const [name, expected] of [
        ['직접 입력 기반 출처 보기: 직접 방문한 서울 😀', '직접 서울 카페에 방문했어요.'],
        ['사진에서 추론 출처 보기: 커피가 놓인 창가', '창가 테이블 위에 커피잔이 보입니다.'],
      ]) {
        const source = article.getByRole('button', { name, exact: true })
        await source.evaluate((node) => node.scrollIntoView({ block: 'center' }))
        const reachable = await source.evaluate((node) => {
          const rect = node.getBoundingClientRect()
          const hit = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2)
          return hit === node || node.contains(hit)
        })
        assert.equal(reachable, true, 'Dock must not obstruct scrolled source: ' + name)
        if (display.touch) await source.tap()
        else await source.click()
        await dialog.waitFor()
        assert.ok((await dialog.innerText()).includes(expected))
        await dialog.getByRole('button', { name: '닫기', exact: true }).click()
        await dialog.waitFor({ state: 'hidden' })
        assert.equal(await source.evaluate((node) => node === document.activeElement), true)
        reachedSources.push({ name, reachable, pointer: display.touch ? 'touch' : 'mouse' })
      }
      await page.evaluate(() => window.scrollTo(0, 0))
      const textBefore = await article.evaluate((node) =>
        [...node.querySelectorAll('[role="button"]')].map((n) => n.textContent).join(''),
      )
      const measurement = await page.evaluate(() => {
        const canvas = document.createElement('canvas')
        canvas.width = canvas.height = 1
        const context = canvas.getContext('2d')
        const rgb = (color) => {
          context.clearRect(0, 0, 1, 1)
          context.fillStyle = color
          context.fillRect(0, 0, 1, 1)
          return [...context.getImageData(0, 0, 1, 1).data].slice(0, 3)
        }
        const luminance = (values) =>
          values
            .map((n) => n / 255)
            .map((n) => (n <= 0.04045 ? n / 12.92 : ((n + 0.055) / 1.055) ** 2.4))
            .reduce((sum, n, i) => sum + n * [0.2126, 0.7152, 0.0722][i], 0)
        const roles = ['owner', 'visual', 'ai', 'unconfirmed'].map((role) => {
          const node = document.querySelector('.text-origin-' + role + '-foreground')
          const style = getComputedStyle(node)
          const fg = luminance(rgb(style.color))
          const bg = luminance(rgb(style.backgroundColor))
          return {
            role,
            ratio: (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05),
            color: style.color,
            background: style.backgroundColor,
          }
        })
        return {
          overflow: document.documentElement.scrollWidth > innerWidth,
          documentWidth: document.documentElement.scrollWidth,
          viewportWidth: innerWidth,
          innerScrollers: [...document.querySelectorAll('main *')].filter(
            (node) =>
              ['auto', 'scroll'].includes(getComputedStyle(node).overflowY) &&
              node.scrollHeight > node.clientHeight,
          ).length,
          roles,
          proseMinFont: Math.min(
            ...[
              ...document.querySelectorAll(
                'article[aria-label="생성된 글"] p [data-writing-origin-phrase], article[aria-label="생성된 글"] li [data-writing-origin-phrase], article[aria-label="생성된 글"] blockquote [data-writing-origin-phrase]',
              ),
            ]
              .filter(
                (node) =>
                  !node.closest('header') &&
                  (node.closest('li') ||
                    node.closest('blockquote') ||
                    node.closest('p')?.textContent.startsWith('서울 😀')),
              )
              .map((node) => Number.parseFloat(getComputedStyle(node).fontSize)),
          ),
          fieldFonts: [
            ...document.querySelectorAll(
              'article[aria-label="생성된 글"] [data-writing-origin-phrase]',
            ),
          ].map((node) => ({
            text: node.textContent,
            fontSize: Number.parseFloat(getComputedStyle(node).fontSize),
          })),
        }
      })
      assert.ok(measurement.proseMinFont >= 16)
      assert.equal(measurement.overflow, false)
      assert.equal(measurement.innerScrollers, 0)
      for (const role of measurement.roles) assert.ok(role.ratio >= 4.5, JSON.stringify(role))
      await page.screenshot({
        animations: 'disabled',
        path: `${output}/${theme}-${display.width}${display.zoom ? '-zoom200' : ''}.png`,
        fullPage: true,
      })
      await page.getByRole('button', { name: '1번째 블록 수정' }).click()
      const input = page.getByRole('textbox', { name: '1번째 블록 내용' })
      await input.evaluate((node) => node.scrollIntoView({ block: 'center' }))
      const inputReachable = await input.evaluate((node) => {
        const rect = node.getBoundingClientRect()
        return document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2) === node
      })
      assert.equal(inputReachable, true, 'Dock must not obstruct active editor input')
      assert.ok(
        (await input.evaluate((node) => Number.parseFloat(getComputedStyle(node).fontSize))) >=
          (display.touch ? 16 : 14),
      )
      await input.fill(finalParagraph)
      await input.evaluate((node) => {
        node.focus()
        node.select()
      })
      const inputCopy = await readNativeCopy(page, input, true)
      assert.equal(inputCopy['text/plain'], finalParagraph)
      nativeClipboard.push(inputCopy)
      await input.evaluate((node) => {
        node.dataset.continuity = 'original'
        node.setSelectionRange(5, 5)
      })
      await page.getByRole('checkbox', { name: '출처 보기' }).click()
      const retained = await input.evaluate((node) => ({
        marker: node.dataset.continuity,
        value: node.value,
        caret: node.selectionStart,
      }))
      assert.deepEqual(retained, { marker: 'original', value: finalParagraph, caret: 5 })
      await page.getByRole('checkbox', { name: '출처 보기' }).click()
      assert.equal(await article.locator('.text-origin-owner-foreground').count(), 0)
      await page.getByText('저장됨', { exact: true }).waitFor()
      assert.equal(saveRequests.at(-1).content.blocks[0].content, finalParagraph)
      assert.equal(post.content.blocks[0].content, finalParagraph)
      assert.equal(await article.locator('.text-origin-owner-foreground').count(), 0)
      assert.ok((await article.locator('.text-origin-unconfirmed-foreground').count()) > 0)
      await article.getByRole('button', { name: '저장', exact: true }).click()
      const currentParagraph = article.locator('p').filter({ hasText: finalParagraph }).first()
      nativeClipboard.push(await cleanNativeCopy(page, currentParagraph, finalParagraph))
      assert.equal(finalizeRequests.length, 0)
      const finalize = page.getByRole('button', { name: '확정하기', exact: true })
      assert.equal(await reachable(finalize), true)
      await finalize.click()
      await page.getByRole('heading', { name: '내보내기', exact: true }).waitFor()
      assert.equal(finalizeRequests.length, 1)
      assert.ok(calls.indexOf('SavePostContent') < calls.indexOf('FinalizePost'))
      const exports = await exportCanonical(page, finalParagraph)
      const hitEvidence = await page.evaluate(() => window.__hitEvidence)
      retainMissingOrigins = true
      post = {
        ...post,
        slug: 't646-legacy',
        status: 'review',
        contentOrigins: undefined,
        finalizedRevision: '0',
      }
      await page.goto(`${url}/posts/${post.slug}`, { waitUntil: 'commit' })
      const legacyArticle = page.getByRole('article', { name: '생성된 글' })
      await legacyArticle.waitFor()
      assert.equal(await legacyArticle.locator('.text-origin-owner-foreground').count(), 0)
      assert.ok((await legacyArticle.locator('.text-origin-unconfirmed-foreground').count()) > 0)
      assert.ok((await legacyArticle.innerText()).includes(finalParagraph))
      await page.getByRole('button', { name: '1번째 블록 수정', exact: true }).click()
      const legacyInput = page.getByRole('textbox', { name: '1번째 블록 내용', exact: true })
      await legacyInput.fill(`${finalParagraph} 레거시 소유자 수정.`)
      await page.getByText('저장됨', { exact: true }).waitFor()
      assert.equal(post.contentOrigins, undefined)
      await legacyArticle.getByRole('button', { name: '저장', exact: true }).click()
      assert.equal(finalizeRequests.length, 1)
      await page.getByRole('button', { name: '확정하기', exact: true }).click()
      await page.getByRole('heading', { name: '내보내기', exact: true }).waitFor()
      assert.equal(finalizeRequests.length, 2)
      const legacyExports = await exportCanonical(page, `${finalParagraph} 레거시 소유자 수정.`)
      const exportScreenshot = `${output}/${theme}-${display.width}${display.zoom ? '-zoom200' : ''}-export.png`
      await page.screenshot({ animations: 'disabled', path: exportScreenshot, fullPage: true })
      assert.equal(
        calls.some((name) => /Start|Generate|Complete|Apply/.test(name)),
        false,
      )
      assert.ok(textBefore.includes('서울 😀'))
      assert.deepEqual(errors, [])
      results.push({
        theme,
        display,
        pointer,
        measurement,
        sourceScreenshot,
        sourceGeometry,
        retained,
        selected,
        reachedSources,
        inputReachable,
        toggleReachable,
        finderReachable,
        aiSourceDetail: true,
        hitEvidence,
        nativeClipboard,
        technical,
        saveRequests,
        finalizeRequests,
        exports,
        legacyExports,
        exportScreenshot,
        errors,
        calls,
      })
      console.log(
        JSON.stringify({
          theme,
          width: display.width,
          zoom: display.zoom ?? 1,
          minContrast: Math.min(...measurement.roles.map((role) => role.ratio)),
          overflow: measurement.overflow,
          innerScrollers: measurement.innerScrollers,
        }),
      )
      await context.close()
    }
  }
  for (const theme of process.env.T646_BROWSER_SMOKE ? ['light'] : ['light', 'dark'])
    for (const width of process.env.T646_BROWSER_SMOKE ? [360] : [360, 1440])
      blindResults.push(await inspectBlind(browser, theme, width))
  const sourcePaths = execFileSync(
    'git',
    [
      'ls-files',
      '--cached',
      '--others',
      '--exclude-standard',
      '--',
      'frontend/src/features/review-writing-origins',
      'frontend/src/features/inspect-writing-request',
      'frontend/src/entities/request-inspection',
      'frontend/src/entities/post/model/origin-projection.ts',
      'frontend/src/entities/post/model/origins.ts',
      'frontend/src/entities/post/ui/BlockList.tsx',
      'frontend/src/entities/post/api/useSavePostContent.ts',
      'frontend/src/shared/ui/selectable-text',
      'frontend/src/shared/lib/clipboard',
      'frontend/src/features/edit-post-content',
      'frontend/src/pages/editor/ui',
      'frontend/src/widgets/export-panel',
      'frontend/src/features/export-naver',
      'frontend/src/features/export-tistory',
      'frontend/src/features/export-site',
      'frontend/src/features/export-markdown',
      'frontend/src/app/styles/index.css',
      'frontend/src/app/providers/i18n/resources/index.ts',
      'frontend/src/test/fixtures/ownerControlContent.ts',
      'frontend/src/widgets/writing-test/ui/WritingTestPair.tsx',
      'frontend/src/entities/writing-test/api/mappers.ts',
    ],
    { encoding: 'utf8' },
  )
    .trim()
    .split('\n')
    .filter((path) => path && !path.includes('.test.') && !path.includes('__snapshots__'))
  sourcePaths.push(
    'frontend/src/pages/editor/lib/origin-inspection-qualification.browser.mjs',
    'frontend/dist/index.html',
  )
  sourcePaths.push(
    ...execFileSync('rg', ['--files', 'frontend/dist/assets'], { encoding: 'utf8' })
      .trim()
      .split('\n')
      .filter((path) => /\.(js|css)$/.test(path)),
  )
  const sourceFiles = await Promise.all(
    sourcePaths.map(async (path) => ({
      path,
      sha256: createHash('sha256')
        .update(await readFile(path))
        .digest('hex'),
    })),
  )
  assert.deepEqual(
    await hashFiles(productionPaths),
    productionFingerprintBefore,
    'Served production assets changed during qualification',
  )
  assert.deepEqual(
    await hashFiles(productSourcePaths),
    sourceFingerprintBefore,
    'Frontend source changed during qualification',
  )
  const screenshotPaths = execFileSync('rg', ['--files', output], { encoding: 'utf8' })
    .trim()
    .split('\n')
    .filter((path) => path.endsWith('.png'))
    .sort()
  const screenshots = await hashFiles(screenshotPaths)
  await writeFile(
    output + '/measurements.json',
    JSON.stringify(
      {
        browserVersion: browser.version(),
        nodeVersion: process.version,
        generatedAt: new Date().toISOString(),
        sourceRevision: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
        sourceFiles,
        productionFingerprintBefore,
        screenshots,
        fixture: 'Synthetic intercepted owner RPCs; no backend/provider calls',
        zoomMethod:
          '200% desktop reflow emulated at720x450 CSS pixels withdeviceScaleFactor2 for1440x900 physical pixels',
        results,
        blindResults,
        limits: [
          'Deterministic synthetic RPC responses exercise the current production frontend; actual persistence/API/queue guarantees are covered separately by backend integration tests.',
          'Chromium headless OS clipboard was read through navigator.clipboard.read after a real keyboard copy; software keyboard and native mobile OS browsers were not exercised.',
          '200% desktop document reflow is emulated at720x450 CSS pixels with DPR2 rather than native browser-menu zoom.',
          'No provider backend, deployment, paid model, live source-accuracy/prose-quality or owner traffic evaluation was run.',
        ],
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    JSON.stringify({
      cases: results.length,
      blindCases: blindResults.length,
      themes: 2,
      viewports: [320, 360, 390, 430, 1440, 1920],
      zoom: '200% reflow',
      output,
    }),
  )
} finally {
  await browser.close()
}
