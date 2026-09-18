import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipProjectSchema } from '@/shared/api'
import { toClipProject } from '@/entities/clip-project'
import { FinalizeClipAction } from './FinalizeClipAction'

afterEach(cleanup)

it.each([
  ['finalized', '이미 확정된 클립이에요.'],
  ['busy', '작업이 끝난 뒤 확정할 수 있어요.'],
  ['invalid_plan', '편집 내용의 오류를 수정하고 다시 렌더해 주세요.'],
  ['unavailable', '확정 가능 여부를 확인하지 못했어요. 화면을 새로고침해 주세요.'],
] as const)('states %s beside the refused control and opens no dialog', (refusal, message) => {
  const project = toClipProject(
    create(ClipProjectSchema, {
      id: 'clip',
      ratio: 'vertical',
      editPlanRevision: 1,
      renderedPlanRevision: 1,
      canFinalize: true,
      result: { id: 'render' },
    }),
  )
  project.finalizationRefusal = refusal
  const action = {
    prepare: vi.fn(async () => project),
    confirm: vi.fn(async () => {}),
    checkAgain: vi.fn(async () => {}),
    pending: false,
    uncertain: false,
    failure: undefined,
    busy: false,
  }
  render(<FinalizeClipAction action={action} project={project} disabled={false} />)
  const button = screen.getByRole('button', { name: '확정하기' })
  expect(button).toBeDisabled()
  expect(button).toHaveAccessibleDescription(message)
  fireEvent.click(button)
  expect(action.prepare).not.toHaveBeenCalled()
  expect(action.confirm).not.toHaveBeenCalled()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.queryByText(/확정하면 원본을 삭제/)).not.toBeInTheDocument()
})
