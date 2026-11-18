import { ProtoConfigurationKind, ProtoAuthoringDraftState } from '@/shared/api'
import { expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

// A new video template is made by talking to the AI from scratch; only one started from a clip
// keeps the seeded wizard (its own feature tests cover that wizard).
it('opens the AI conversation for a new video template without a session or AI work', async () => {
  const calls: string[] = []
  renderAppAt('/video-templates/new', { user: { id: 'alice' }, calls })
  expect(
    await screen.findByRole('textbox', { name: '어떤 영상인지 말해 주세요' }),
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: '어떤 체험을 영상으로 남기나요?' }),
  ).not.toBeInTheDocument()
  expect(calls.filter((call) => /^(Create|Start)/.test(call))).toEqual([])
})

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
  await screen.findByRole('textbox', { name: '바꾸고 싶은 점을 말해 주세요' })
  expect(calls).toContain('GetAuthoringSession')
  expect(calls.filter((call) => /^(Create|Start)/.test(call))).toEqual([])
})
