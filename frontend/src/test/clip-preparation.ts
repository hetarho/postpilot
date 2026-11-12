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
