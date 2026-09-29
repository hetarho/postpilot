// The routed editor's shared test harness: the six EditorPage suites drive the same page, step by
// step, so the ways to reach a step, the brief and the dock fields live here once (ARCH-16).
import { cleanup, screen, waitFor } from '@testing-library/react'
import type userEvent from '@testing-library/user-event'
import { expect, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { discardUploadBatches } from '@/features/upload-photos'
import type { FakeQualityReading } from './quality'

export const USER = { id: 'alice' }

const originalClipboard = navigator.clipboard

/** What every editor suite undoes after each case. `clearCaret` is a deep import, so each suite
 *  calls it beside this. */
export function resetEditorTest(): void {
  cleanup()
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: originalClipboard })
  discardUploadBatches()
  initializeI18n('ko')
  vi.unstubAllGlobals()
}

/** The editor is one mounted component showing three step panels, so a test reaches a panel the
 *  post's status did not open by pressing its step. */
export async function openStep(user: ReturnType<typeof userEvent.setup>, label: string) {
  await user.click(await screen.findByRole('tab', { name: label }))
}

/** 글 다듬기's one way out, 확정하기, stands at the top-right of the revision row and finalizes at
 *  once (POST-56): no popover or modal stands between the press and the run. */
export async function finalize(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '확정하기' }))
}

/** The writing brief — 관찰/작성 모델, 작성 A/B 후보, 목표 언어, 목표 분량 — lives behind ONE trigger
 *  in the dock (POST-51), so a test that drives any of them opens it first. 말투 and 템플릿 are the
 *  exceptions and ride the dock's own row; see `dockField` below. */
export const BRIEF_TRIGGER = /^(글쓰기 옵션|Writing options)$/
const GENERATE_STEP = /^(글 생성|Generate)$/
export const generateTab = () =>
  screen.queryAllByRole('tab').find((tab) => GENERATE_STEP.test(tab.textContent ?? ''))
export async function openBrief(user: ReturnType<typeof userEvent.setup>) {
  // Wait for the editor to have its post: the lifecycle bar exists only once it does, and the
  // brief belongs to that bar's first step. `/posts/new` has no bar, so its trigger is already up.
  await waitFor(() =>
    expect(screen.queryByRole('button', { name: BRIEF_TRIGGER }) ?? generateTab()).toBeDefined(),
  )
  const generate = generateTab()
  if (generate && generate.getAttribute('aria-selected') !== 'true') await user.click(generate)
  const trigger = await screen.findByRole('button', { name: BRIEF_TRIGGER })
  if (trigger.getAttribute('aria-expanded') !== 'true') await user.click(trigger)
  return screen.getByRole('dialog', { name: BRIEF_TRIGGER })
}

/** A field inside the brief. Its accessible name is "<label> <current value>" — the WAI-APG
 *  select-only combobox shape — so the label is a prefix, not the whole name. */
export async function briefField(user: ReturnType<typeof userEvent.setup>, label: string) {
  await openBrief(user)
  return screen.findByRole('combobox', { name: new RegExp(label) })
}

/** 말투 and 템플릿 are the two parts of the brief that are NOT behind the trigger: both ride the
 *  dock's own row beside the glyph, so a wrong voice or template is visible without opening
 *  anything. */
export async function dockField(user: ReturnType<typeof userEvent.setup>, label: RegExp) {
  // Same wait as `openBrief`: the lifecycle bar exists only once the editor has its post, and the
  // dock row these fields ride belongs to that bar's first step.
  await waitFor(() =>
    expect(screen.queryByRole('combobox', { name: label }) ?? generateTab()).toBeDefined(),
  )
  const generate = generateTab()
  if (generate && generate.getAttribute('aria-selected') !== 'true') await user.click(generate)
  return screen.findByRole('combobox', { name: label })
}
export const voiceField = (user: ReturnType<typeof userEvent.setup>) => dockField(user, /말투/)
export const templateField = (user: ReturnType<typeof userEvent.setup>) => dockField(user, /템플릿/)

/** jsdom has no image decoder, canvas encoder or object URLs; these stand in for the
 *  browser so the test can follow a file through the whole upload handshake. */
export function stubBrowserImagePipeline() {
  vi.stubGlobal(
    'createImageBitmap',
    vi.fn(async () => ({ width: 4032, height: 3024, close: () => {} })),
  )
  vi.stubGlobal(
    'OffscreenCanvas',
    class {
      getContext() {
        return { fillRect() {}, drawImage() {}, fillStyle: '' }
      }
      convertToBlob = async () => new Blob(['jpeg'], { type: 'image/jpeg' })
    },
  )
  // jsdom's URL lacks these two; the class itself must stay (the router constructs URLs).
  URL.createObjectURL = () => 'blob:preview'
  URL.revokeObjectURL = () => {}
  const put = vi.fn(async () => new Response(null, { status: 200 }))
  vi.stubGlobal('fetch', put)
  return { put }
}

/** The account's M1 over band, so the brief's quality rows offer a tick. */
export const M1_OVER_BAND: FakeQualityReading[] = [
  {
    metric: 'title_saturation',
    verdict: 'over_band',
    minimum: 10,
    publishedCount: 12,
    ruleText: '제목마다 “성수 카페”를 반복하지 마세요.',
    values: { share: 0.42, shareWarnAbove: 0.3 },
  },
]
