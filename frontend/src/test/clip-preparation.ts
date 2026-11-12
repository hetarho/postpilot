import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

/** Navigate the actual retained guide. New/finalized and focused-job views keep their own UI. */
export async function clipGuideTo(part: 'sources' | 'story' | 'style' | 'review') {
  await screen.findByRole('link', { name: /돌아가기|Back to/ })
  // The return link is also painted while GetClipProject is pending. Wait for the
  // actual workspace before deciding whether this creation has a retained guide.
  await waitFor(() => {
    if (
      screen.queryByRole('navigation', { name: /클립 만들기 단계|Clip steps/ }) ||
      screen.queryByRole('tablist', { name: /클립 단계|Clip steps/ }) ||
      screen.queryByRole('button', { name: /^(클립 만들기|Create clip)$/ })
    )
      return
    throw new Error('Clip workspace is still loading')
  })
  const progress = screen.queryByRole('navigation', { name: /클립 만들기 단계|Clip steps/ })
  if (!progress) return
  const names = {
    sources: /^(소스|Sources)$/,
    story: /^(이야기|Story)$/,
    style: /^(스타일|Style)$/,
    review: /^(확인|Review)$/,
  }
  const target = within(progress).getByRole('button', { name: names[part] })
  if (target.getAttribute('aria-current') !== 'step') await userEvent.click(target)
}
