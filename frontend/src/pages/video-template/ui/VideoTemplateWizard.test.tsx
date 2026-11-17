import { ProtoConfigurationKind, ProtoAuthoringDraftState } from '@/shared/api'
import { expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

it('resumes an existing unsaved video authoring session without a new creation request', async () => {
  const calls: string[] = []
  const artifact = {
    id: 'draft',
    name: '이어 만들기',
    description: '',
    titleArea: '',
    body: '<clip version="1"/>',
    introPreset: 'a',
    outroPreset: 'b',
    allowedCaptionStyles: [],
  }
  renderAppAt('/video-templates/new?session=legacy', {
    user: { id: 'alice' },
    calls,
    authoring: {
      sessions: [
        {
          id: 'legacy',
          kind: ProtoConfigurationKind.VIDEO_TEMPLATE,
          phase: 'editing',
          revision: 2,
          workingSource: artifact,
          selected: artifact,
          hasUnpublishedChanges: true,
          draftState: ProtoAuthoringDraftState.VALID,
        },
      ],
    },
  })
  await screen.findByRole('textbox', { name: '무엇을 바꿀지 말해 주세요' })
  expect(calls).toContain('GetAuthoringSession')
  expect(calls.filter((call) => /^(Create|Start)/.test(call))).toEqual([])
  expect(
    screen.queryByRole('heading', { name: '어떤 체험을 영상으로 남기나요?' }),
  ).not.toBeInTheDocument()
})
