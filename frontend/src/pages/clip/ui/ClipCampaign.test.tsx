import { afterEach, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { clipGuideTo } from '@/test/clip-preparation'
import { emptyClipProject, type ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const project = {
  id: 'project',
  title: '제주 여행',
  videoTemplateId: '',
  ratio: 'vertical' as const,
  targetDurationMs: 30000,
  disclosure: 'ad' as const,
}
afterEach(() => discardClipDraftQueues())

const sponsored = () => screen.getByRole('button', { name: /체험단으로 다녀왔어요/ })
const selfPaid = () => screen.getByRole('button', { name: /내 돈으로 다녀왔어요/ })

it('asks 영상 종류 first and edits the guide on 요청, read through real navigation', async () => {
  const user = userEvent.setup()
  const writes: ClipProjectDraft[] = []
  const calls: string[] = []
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [project], projectWrites: writes, calls },
  })
  expect(await screen.findByRole('heading', { name: '어떤 영상을 만드시나요?' })).toBeVisible()
  expect(selfPaid()).toHaveAttribute('aria-pressed', 'true')
  expect(screen.queryByRole('button', { name: /^요청$/ })).not.toBeInTheDocument()
  await user.click(sponsored())
  await clipGuideTo('requirements')
  fireEvent.change(await screen.findByLabelText('광고주 가이드'), {
    target: { value: '  광고주가 요청한 안내\n키워드: 대표 메뉴\n#제주  ' },
  })
  // The clip's items stand expanded under 영상에 꼭 담을 내용; chips offer only keywords and tags.
  expect(screen.getByText('영상에 꼭 담을 내용')).toBeVisible()
  expect(screen.queryByRole('button', { name: '셀 수 있는 항목' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '키워드 · 대표 메뉴 추가' }))
  await user.type(screen.getByLabelText('필수 해시태그'), '#제주{Enter}')
  expect(screen.queryByLabelText('최소 글자 수')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('최소 사진 수')).not.toBeInTheDocument()
  await waitFor(
    () =>
      expect(writes.at(-1)).toMatchObject({
        campaign: true,
        advertiserGuide: {
          text: '광고주가 요청한 안내\n키워드: 대표 메뉴\n#제주',
          keywords: ['대표 메뉴'],
          hashtags: ['제주'],
        },
      }),
    { timeout: 4000 },
  )
  await clipGuideTo('kind')
  await user.click(selfPaid())
  expect(screen.queryByRole('button', { name: /^요청$/ })).not.toBeInTheDocument()
  await waitFor(() => expect(writes.at(-1)?.campaign).toBe(false), { timeout: 4000 })
  expect(writes.at(-1)?.advertiserGuide?.text).toBe(
    '광고주가 요청한 안내\n키워드: 대표 메뉴\n#제주',
  )
  await user.click(sponsored())
  await clipGuideTo('requirements')
  expect(await screen.findByLabelText('광고주 가이드')).toHaveValue(
    '광고주가 요청한 안내\n키워드: 대표 메뉴\n#제주',
  )
  await waitFor(() => expect(writes.at(-1)?.campaign).toBe(true), { timeout: 4000 })
  expect(calls.some((call) => /Quote|Start/.test(call))).toBe(false)
})

// The autosave's answer once re-rendered ① against the project from before the save: the guide
// unmounted for a render and came back with an item being typed gone.
it('keeps the guide and a half-typed item while its autosave lands', async () => {
  const user = userEvent.setup()
  const writes: ClipProjectDraft[] = []
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [{ ...project, campaign: true }], projectWrites: writes },
  })
  await clipGuideTo('requirements')
  fireEvent.change(await screen.findByLabelText('광고주 가이드'), {
    target: { value: '광고주가 요청한 안내' },
  })
  const keyword = screen.getByLabelText('필수 키워드')
  await user.type(keyword, '대표')
  await waitFor(
    () =>
      expect(writes.at(-1)).toMatchObject({
        campaign: true,
        advertiserGuide: { text: '광고주가 요청한 안내', keywords: [] },
      }),
    { timeout: 4000 },
  )
  // The status turns 저장됨 only once that save's answer has reached the form.
  expect(await screen.findByText('저장됨', undefined, { timeout: 4000 })).toBeInTheDocument()
  expect(keyword).toBeInTheDocument()
  expect(keyword).toHaveValue('대표')
  await user.type(keyword, ' 메뉴{Enter}')
  expect(screen.getByLabelText('필수 해시태그')).toBeInTheDocument()
  await waitFor(() => expect(writes.at(-1)?.advertiserGuide?.keywords).toEqual(['대표 메뉴']), {
    timeout: 4000,
  })
})

it('starts a new clip sponsored and reads the stored choice, guide and tracker', async () => {
  expect(emptyClipProject().campaign).toBe(true)
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: {
      projects: [
        {
          ...project,
          campaign: true,
          advertiserGuide: { text: '보관된 요청', keywords: ['제주 여행'], hashtags: ['제주'] },
        },
      ],
    },
  })
  await clipGuideTo('kind')
  expect(sponsored()).toHaveAttribute('aria-pressed', 'true')
  await clipGuideTo('requirements')
  expect(screen.getByLabelText('광고주 가이드')).toHaveValue('보관된 요청')
  await clipGuideTo('content')
  const tracker = screen.getByRole('complementary', { name: '광고주 요청 챙기기' })
  // The title holds the keyword, so the tracker counts it as held.
  expect(tracker).toHaveTextContent('입력한 키워드 1 / 1')
})
