import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

/** Navigate the actual retained guide, `/clips/new`'s unminted one included. Finalized and
 *  focused-job views keep their own UI. */
export type ClipGuidePart =
  'kind' | 'requirements' | 'sources' | 'template' | 'content' | 'style' | 'note' | 'review'
export async function clipGuideTo(part: ClipGuidePart) {
  await screen.findByRole('link', { name: /돌아가기|Back to/ })
  // The return link is also painted while GetClipProject is pending. Wait for the
  // actual workspace before deciding whether this creation has a retained guide.
  await waitFor(() => {
    if (
      screen.queryByRole('navigation', { name: /클립 만들기 단계|Clip steps/ }) ||
      screen.queryByRole('tablist', { name: /클립 단계|Clip steps/ })
    )
      return
    throw new Error('Clip workspace is still loading')
  })
  // From ② 관찰, where `clipGuideToActions` leaves a test, the guide is one step back on ①.
  const observe = screen.queryByRole('tab', { name: /^(관찰|Observe)$/ })
  if (observe?.getAttribute('aria-selected') === 'true')
    await userEvent.click(screen.getByRole('tab', { name: /^(생성|Create)$/ }))
  const progress = screen.queryByRole('navigation', { name: /클립 만들기 단계|Clip steps/ })
  if (!progress) return
  const names: Record<ClipGuidePart, RegExp> = {
    kind: /^(영상 종류|Video type)$/,
    requirements: /^(요청|Requests)$/,
    sources: /^(영상|Footage)$/,
    template: /^(템플릿|Template)$/,
    content: /^(내용|Content)$/,
    style: /^(스타일|Style)$/,
    note: /^(덧붙임|Note)$/,
    review: /^(확인|Review)$/,
  }
  const target = within(progress).getByRole('button', { name: names[part] })
  if (target.getAttribute('aria-current') !== 'step') await userEvent.click(target)
}

/** ②'s step bar holds the generation actions (CLIP-40): pass ①'s 확인 and open ② 관찰. */
export async function clipGuideToActions() {
  await clipGuideTo('review')
  const tab = screen.queryByRole('tab', { name: /^(관찰|Observe)$/ })
  if (tab && tab.getAttribute('aria-selected') !== 'true') await userEvent.click(tab)
}
