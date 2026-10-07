import { chromium } from 'playwright'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import assert from 'node:assert/strict'

const output = process.env.T642_BROWSER_OUTPUT ?? '/tmp/postpilot-t642-browser'
const url = process.env.T642_WEB_URL ?? 'http://127.0.0.1:26142'
await mkdir(output, { recursive: true })
const content = {
  title: '서울 😀에서 찾은 카페',
  summary: '직접 방문한 기록과 사진 속 풍경',
  tags: ['서울카페', '커피'],
  blocks: [
    {
      type: 'TEXT',
      content: '서울 😀에서 커피 사진을 보고 행복을 상상했어요. 서울 😀의 기록입니다.',
    },
    { type: 'HEADING', content: '사진으로 보는 커피', level: 2 },
    { type: 'QUOTE', content: '내가 남긴 카페 기록' },
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
  { id: 'memo', kind: 'memo', text: '직접 서울 카페에 방문했어요.', available: true },
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
const browser = await chromium.launch({ headless: true })
const results = []
try {
  for (const theme of ['light', 'dark']) {
    for (const display of [
      { width: 320, height: 640, touch: true },
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
      await context.addInitScript((theme) => {
        localStorage.setItem('postpilot.theme', theme)
        localStorage.setItem('postpilot.locale', 'ko')
        localStorage.setItem(
          'postpilot.setup.v1.t642-browser',
          JSON.stringify({
            version: 1,
            ownerId: 't642-browser',
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
      page.on('pageerror', (error) => errors.push(error.message))
      let post = {
        slug: 't642-origins',
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
            viewUrl: '/t642-photo.svg',
          },
          {
            id: 'photo-b',
            filename: 'window.jpg',
            width: 640,
            height: 320,
            viewUrl: '/t642-photo.svg',
          },
        ],
        videos: [{ id: 'video-a', filename: 'cafe.mp4' }],
      }
      await page.route('**/t642-photo.svg', (route) =>
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
        let response = {}
        if (method === 'GetMe')
          response = {
            user: {
              id: 't642-browser',
              email: 'fixture@example.test',
              emailVerified: true,
              hasPassword: true,
            },
            plan: 'PLAN_FREE',
          }
        if (method === 'GetMyPlan')
          response = { plan: 'PLAN_FREE', balance: { credits: 0, lots: [] } }
        if (method === 'GetPost') response = { post }
        if (method === 'SavePostContent') {
          const request = route.request().postDataJSON()
          post = {
            ...post,
            content: request.content,
            contentRevision: String(Number(post.contentRevision) + 1),
            contentHash: 'saved',
            contentOrigins: undefined,
          }
          response = { post }
        }
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify(response) })
      })
      await page.goto(url + '/posts/t642-origins', { waitUntil: 'commit' })
      const article = page.getByRole('article', { name: '생성된 글' })
      try {
        await article.waitFor({ timeout: 15000 })
      } catch (error) {
        console.log(JSON.stringify({ body: await page.locator('body').innerText(), errors, calls }))
        throw error
      }
      await page.evaluate(() => document.fonts.ready)
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
      const phrase = article
        .getByRole('button', { name: '직접 입력 기반 출처 보기: 서울 😀', exact: true })
        .first()
      await phrase.focus()
      await page.keyboard.press('Enter')
      const dialog = page.getByRole('dialog')
      await dialog.waitFor()
      assert.ok((await dialog.innerText()).includes('직접 서울 카페에 방문했어요.'))
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
      await page.evaluate(() => getSelection().removeAllRanges())
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
        }
      })
      assert.equal(measurement.overflow, false)
      assert.equal(measurement.innerScrollers, 0)
      for (const role of measurement.roles) assert.ok(role.ratio >= 4.5, JSON.stringify(role))
      await page.screenshot({
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
      await input.fill('한글 😀 입력을 이어갑니다.')
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
      assert.deepEqual(retained, {
        marker: 'original',
        value: '한글 😀 입력을 이어갑니다.',
        caret: 5,
      })
      await page.getByRole('checkbox', { name: '출처 보기' }).click()
      assert.equal(await article.locator('.text-origin-owner-foreground').count(), 0)
      await page.getByRole('button', { name: '취소', exact: true }).click()
      assert.equal(
        await article
          .getByRole('button', { name: '직접 입력 기반 출처 보기: 서울 😀', exact: true })
          .count(),
        2,
      )
      assert.ok(textBefore.includes('서울 😀'))
      assert.equal(
        calls.some((name) => /Start|Generate|Complete|Finalize/.test(name)),
        false,
      )
      assert.deepEqual(errors, [])
      results.push({
        theme,
        display,
        measurement,
        retained,
        selected,
        reachedSources,
        inputReachable,
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
  const sourcePaths = [
    ...new Set([
      ...execFileSync(
        'git',
        ['diff', '--name-only', '--', 'frontend/src', 'scripts/lint-style-escapes.mjs'],
        { encoding: 'utf8' },
      )
        .trim()
        .split('\n'),
      ...execFileSync('git', ['ls-files', '--others', '--exclude-standard', 'frontend/src'], {
        encoding: 'utf8',
      })
        .trim()
        .split('\n'),
    ]),
  ]
    .filter((path) => path && !path.includes('.test.'))
    .sort()
  const sourceFiles = await Promise.all(
    sourcePaths.map(async (path) => ({
      path,
      sha256: createHash('sha256')
        .update(await readFile(path))
        .digest('hex'),
    })),
  )
  await writeFile(
    output + '/measurements.json',
    JSON.stringify(
      {
        sourceRevision: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
        sourceFiles,
        fixture: 'Synthetic intercepted owner RPCs; no backend/provider calls',
        zoomMethod:
          '200% desktop reflow emulated at720x450 CSS pixels withdeviceScaleFactor2 for1440x900 physical pixels',
        results,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    JSON.stringify({
      cases: results.length,
      themes: 2,
      viewports: [320, 390, 1440, 1920],
      zoom: '200% reflow',
      output,
    }),
  )
} finally {
  await browser.close()
}
