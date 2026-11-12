import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipProjectSchema } from '@/shared/api'
import { toClipProject, type ClipProject } from '@/entities/clip-project'
import { ClipPublishedUrlField } from './ClipPublishedUrlField'

afterEach(cleanup)

function clip(overrides: Record<string, unknown> = {}) {
  return toClipProject(
    create(ClipProjectSchema, {
      id: 'clip',
      ratio: 'vertical',
      channel: 'naver_clip',
      editPlanRevision: 1,
      renderedPlanRevision: 1,
      confirmedPlanRevision: 1,
      canFinalize: true,
      result: { id: 'render' },
      ...overrides,
    }),
  )
}

function actionFor(project: ClipProject) {
  return {
    prepare: vi.fn(async () => project),
    confirm: vi.fn<(url?: string) => Promise<void>>(async () => {}),
    checkAgain: vi.fn(async () => {}),
    savePublishedUrl: vi.fn<(url: string) => Promise<boolean>>(async () => true),
    pending: false,
    uncertain: false,
    failure: undefined,
    busy: false,
  }
}

it.each([
  ['finalized', '이미 완료된 클립이에요.'],
  ['busy', '작업이 끝난 뒤 확정할 수 있어요.'],
  ['unconfirmed', '영상 다듬기에서 확정하기를 누른 뒤 완료로 표시할 수 있어요.'],
  ['invalid_plan', '편집 내용의 오류를 수정하고 다시 렌더해 주세요.'],
  ['unavailable', '확정 가능 여부를 확인하지 못했어요. 화면을 새로고침해 주세요.'],
] as const)('states %s beside the refused controls and opens no dialog', (refusal, message) => {
  const project = clip()
  project.finalizationRefusal = refusal
  const action = actionFor(project)
  render(<ClipPublishedUrlField action={action} project={project} disabled={false} />)
  for (const name of ['완료로 표시', 'URL 저장']) {
    const button = screen.getByRole('button', { name })
    expect(button).toBeDisabled()
    expect(button).toHaveAccessibleDescription(message)
    fireEvent.click(button)
  }
  expect(action.prepare).not.toHaveBeenCalled()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('reads an unconfirmed plan as unconfirmed before any render check', () => {
  const project = clip({ confirmedPlanRevision: 0 })
  render(<ClipPublishedUrlField action={actionFor(project)} project={project} disabled={false} />)
  expect(screen.getByRole('button', { name: '완료로 표시' })).toHaveAccessibleDescription(
    '영상 다듬기에서 확정하기를 누른 뒤 완료로 표시할 수 있어요.',
  )
})

it('marks complete through one confirmation, without a URL', async () => {
  const project = clip()
  const action = actionFor(project)
  render(<ClipPublishedUrlField action={action} project={project} disabled={false} />)
  fireEvent.click(screen.getByRole('button', { name: '완료로 표시' }))
  const dialog = within(await screen.findByRole('dialog'))
  expect(dialog.getByText(/완료로 표시하면 원본을 삭제/)).toBeVisible()
  expect(action.confirm).not.toHaveBeenCalled()
  fireEvent.click(dialog.getByRole('button', { name: '완료로 표시' }))
  expect(action.confirm).toHaveBeenCalledWith(undefined)
})

it('finalizes with the posted URL through the same confirmation', async () => {
  const project = clip()
  const action = actionFor(project)
  render(<ClipPublishedUrlField action={action} project={project} disabled={false} />)
  fireEvent.change(screen.getByLabelText('네이버 클립 주소'), {
    target: { value: ' https://clip.naver.com/v/1 ' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'URL 저장' }))
  const dialog = within(await screen.findByRole('dialog'))
  expect(dialog.getByText('게시 URL을 저장하고 클립을 완료할까요?')).toBeVisible()
  fireEvent.click(dialog.getByRole('button', { name: '저장하고 완료' }))
  expect(action.confirm).toHaveBeenCalledWith('https://clip.naver.com/v/1')
})

it('refuses a URL off the channel’s host on the field, before any request', () => {
  const project = clip()
  const action = actionFor(project)
  render(<ClipPublishedUrlField action={action} project={project} disabled={false} />)
  const field = screen.getByLabelText('네이버 클립 주소')
  fireEvent.change(field, { target: { value: 'https://www.instagram.com/reel/1' } })
  fireEvent.click(screen.getByRole('button', { name: 'URL 저장' }))
  expect(field).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByRole('alert')).toBeInTheDocument()
  expect(action.prepare).not.toHaveBeenCalled()
})

it('closing the dialog starts nothing', async () => {
  const project = clip()
  const action = actionFor(project)
  render(<ClipPublishedUrlField action={action} project={project} disabled={false} />)
  fireEvent.click(screen.getByRole('button', { name: '완료로 표시' }))
  const dialog = within(await screen.findByRole('dialog'))
  fireEvent.click(dialog.getByRole('button', { name: /취소/ }))
  expect(action.confirm).not.toHaveBeenCalled()
})

it('replaces a finalized clip’s URL without a dialog or 완료로 표시', async () => {
  const project = clip({
    finalizedAt: '2026-11-12T00:00:00Z',
    finalizedPlanRevision: 1,
    finalizedResultId: 'render',
    publishedUrl: 'https://clip.naver.com/v/1',
  })
  const action = actionFor(project)
  render(<ClipPublishedUrlField action={action} project={project} disabled={false} />)
  expect(screen.queryByRole('button', { name: '완료로 표시' })).not.toBeInTheDocument()
  const field = screen.getByLabelText('네이버 클립 주소')
  expect(field).toHaveValue('https://clip.naver.com/v/1')
  fireEvent.change(field, { target: { value: 'https://naver.me/abc' } })
  fireEvent.click(screen.getByRole('button', { name: 'URL 바꾸기' }))
  expect(action.savePublishedUrl).toHaveBeenCalledWith('https://naver.me/abc')
  expect(await screen.findByText('게시 URL을 저장했어요.')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(action.prepare).not.toHaveBeenCalled()
})
