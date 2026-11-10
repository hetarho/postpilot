import { afterEach, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
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

it('edits the switch and guide on ①', async () => {
  const user = userEvent.setup()
  const writes: ClipProjectDraft[] = []
  const calls: string[] = []
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [project], projectWrites: writes, calls },
  })
  const toggle = await screen.findByRole('switch', { name: '체험단 글' })
  expect(toggle).not.toBeChecked()
  expect(screen.queryByLabelText('광고주 가이드')).not.toBeInTheDocument()
  await user.click(toggle)
  fireEvent.change(await screen.findByLabelText('광고주 가이드'), {
    target: { value: '  광고주가 요청한 안내  ' },
  })
  await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))
  await user.type(screen.getByLabelText('필수 키워드'), '대표 메뉴{Enter}')
  await user.type(screen.getByLabelText('필수 해시태그'), '#제주{Enter}')
  expect(screen.queryByLabelText('최소 글자 수')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('최소 사진 수')).not.toBeInTheDocument()
  await waitFor(
    () =>
      expect(writes.at(-1)).toMatchObject({
        campaign: true,
        advertiserGuide: {
          text: '광고주가 요청한 안내',
          keywords: ['대표 메뉴'],
          hashtags: ['제주'],
        },
      }),
    { timeout: 4000 },
  )
  await user.click(toggle)
  expect(screen.queryByLabelText('광고주 가이드')).not.toBeInTheDocument()
  await waitFor(() => expect(writes.at(-1)?.campaign).toBe(false), { timeout: 4000 })
  expect(writes.at(-1)?.advertiserGuide?.text).toBe('광고주가 요청한 안내')
  await user.click(toggle)
  expect(await screen.findByLabelText('광고주 가이드')).toHaveValue('광고주가 요청한 안내')
  await waitFor(() => expect(writes.at(-1)?.campaign).toBe(true), { timeout: 4000 })
  expect(calls.some((call) => /Quote|Start/.test(call))).toBe(false)
})

// The autosave's answer once re-rendered ① against the project from before the save: the guide
// unmounted for a render and came back with `셀 수 있는 항목` closed and the item being typed gone.
it('keeps the open guide and a half-typed item while its autosave lands', async () => {
  const user = userEvent.setup()
  const writes: ClipProjectDraft[] = []
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [project], projectWrites: writes },
  })
  const toggle = await screen.findByRole('switch', { name: '체험단 글' })
  await user.click(toggle)
  fireEvent.change(await screen.findByLabelText('광고주 가이드'), {
    target: { value: '광고주가 요청한 안내' },
  })
  const counted = screen.getByRole('button', { name: '셀 수 있는 항목' })
  await user.click(counted)
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
  expect(toggle).toBeChecked()
  expect(counted).toBeInTheDocument()
  expect(counted).toHaveAttribute('aria-expanded', 'true')
  expect(keyword).toHaveValue('대표')
  await user.type(keyword, ' 메뉴{Enter}')
  expect(screen.getByLabelText('필수 해시태그')).toBeInTheDocument()
  await waitFor(() => expect(writes.at(-1)?.advertiserGuide?.keywords).toEqual(['대표 메뉴']), {
    timeout: 4000,
  })
})

it('starts a new clip on and reads the stored switch and guide', async () => {
  expect(emptyClipProject().campaign).toBe(true)
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: {
      projects: [
        {
          ...project,
          campaign: true,
          advertiserGuide: { text: '보관된 요청', keywords: [], hashtags: [] },
        },
      ],
    },
  })
  expect(await screen.findByRole('switch', { name: '체험단 글' })).toBeChecked()
  expect(screen.getByLabelText('광고주 가이드')).toHaveValue('보관된 요청')
})
