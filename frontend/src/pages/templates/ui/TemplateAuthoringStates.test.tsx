import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoConfigurationKind, ProtoAuthoringDraftState } from '@/shared/api'
import { renderAppAt } from '@/test/app'
const kind = ProtoConfigurationKind.POST_TEMPLATE
const template = { id: 'saved', name: '저장된 구성', body: '<write>도입</write>', postCount: 2 }
const summary = {
  sessionId: 'editing',
  kind,
  targetId: 'saved',
  displayName: '저장된 구성',
  revision: 3,
  savedAvailable: true,
  hasUnpublishedChanges: true,
  activeJobId: 'active',
  publicationPending: true,
  targetConflict: true,
  draftState: ProtoAuthoringDraftState.INVALID,
  updatedAt: '2026-10-07T00:00:00Z',
}
describe('batched named template states', () => {
  it('marks a tile whose template has AI work running, from one summary batch', async () => {
    const calls: string[] = []
    renderAppAt('/templates', {
      user: { id: 'alice' },
      calls,
      templates: { templates: [template] },
      authoring: { summaries: [summary] },
    })
    // A tile carries the one fact that matters most — here, AI is working on it — and the
    // template's own screen carries the rest.
    const directory = within(await screen.findByRole('region', { name: '저장된 템플릿' }))
    expect(
      await directory.findByText('템플릿 “저장된 구성”의 AI 작업이 진행 중이에요.'),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListAuthoringSummaries')).toHaveLength(1),
    )
    expect(calls).not.toContain('GetAuthoringSession')
    expect(calls).not.toContain('GetLatestAuthoringSession')
  })
  it('separates an unsaved new item from usable saved rows and resumes its exact session', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const draft = {
      id: 'draft',
      name: '아직 만드는 구성',
      body: '<write>요점</write>',
      description: '',
      titleArea: '',
    }
    const { router } = renderAppAt('/templates', {
      user: { id: 'alice' },
      calls,
      templates: { templates: [template] },
      authoring: {
        sessions: [
          {
            id: 'new-work',
            kind,
            targetId: '',
            revision: 2,
            phase: 'editing',
            workingSource: draft,
            selected: draft,
            savedAvailable: false,
            hasUnpublishedChanges: true,
            draftState: ProtoAuthoringDraftState.VALID,
          },
        ],
      },
    })
    const directory = within(await screen.findByRole('region', { name: '저장된 템플릿' }))
    expect(directory.getAllByRole('listitem')).toHaveLength(1)
    const unsaved = within(await screen.findByRole('region', { name: '이어서 만들던 대화' }))
    await user.click(unsaved.getByRole('link', { name: '템플릿 “아직 만드는 구성” 이어서 만들기' }))
    await screen.findByRole('textbox', { name: '바꾸고 싶은 점을 말해 주세요' })
    expect(router.state.location.search).toMatchObject({ session: 'new-work' })
    expect(calls).toContain('GetAuthoringSession')
    expect(calls).not.toContain('CreateAuthoringSession')
    expect(calls).not.toContain('StartAuthoringOperation')
  })
  it('keeps saved rows usable and reports failed summaries instead of claiming no unfinished work', async () => {
    renderAppAt('/templates', {
      user: { id: 'alice' },
      templates: { templates: [template] },
      authoring: { summaryFails: true },
    })
    expect(
      await screen.findByText(
        '템플릿 편집 상태를 확인하지 못했어요. 저장된 템플릿은 계속 사용할 수 있어요.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: new RegExp('^' + template.name) })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
  it('shows the actual last confirmed publication outcome on the template’s own screen', async () => {
    renderAppAt('/templates/saved', {
      user: { id: 'alice' },
      templates: { templates: [template] },
      authoring: {
        summaries: [
          {
            ...summary,
            hasUnpublishedChanges: false,
            activeJobId: '',
            publicationPending: false,
            targetConflict: false,
            lastPublication: { kind, id: 'saved', name: '저장된 구성', outcome: 'updated' },
          },
        ],
      },
    })
    expect(
      await screen.findByText('“저장된 구성” 글 구성의 변경사항을 저장했어요.'),
    ).toBeInTheDocument()
  })
})
