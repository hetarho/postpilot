import { ProtoConfigurationKind, ProtoAuthoringDraftState, Stage } from '@/shared/api'
import { expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { parseClipTemplate, type ClipRecipe } from '@/entities/clip-template'
import { renderAppAt } from '@/test/app'
import { chooseOption } from '@/test/listbox'
import type { FakeTemplatesOptions } from '@/test/templates'

it('creates a 카페 video template end to end', async () => {
  const user = userEvent.setup()
  const writes: ClipRecipe[] = [],
    calls: string[] = []
  const { router } = renderAppAt('/video-templates/new', {
    user: { id: 'alice' },
    calls,
    clips: { templates: [], writes, calls },
  })
  await user.click(await screen.findByRole('button', { name: /^카페/ }))
  await user.click(await screen.findByRole('button', { name: /^디저트/ }))
  expect(
    await screen.findByRole('heading', { name: '이런 영상은 보통 이렇게 이어져요' }),
  ).toBeInTheDocument()
  expect(screen.getAllByRole('checkbox')).toHaveLength(7)
  expect(screen.getByRole('checkbox', { name: /^아쉬운 점/ })).toBeChecked()
  await user.click(screen.getByRole('button', { name: '다음' }))
  await user.click(screen.getByRole('button', { name: '다음' }))
  const hook = screen.getByLabelText('훅 (첫 장면) 단계 이름')
  await user.clear(hook)
  await user.type(hook, '처음 만난 카페')
  await user.click(screen.getByRole('button', { name: '내 말로 쓰기' }))
  for (;;) {
    const heading = await screen.findByRole('heading', {
      level: 2,
      name: /단계$|영상의 디자인을 골라요/,
    })
    if (heading.textContent === '영상의 디자인을 골라요') break
    if (heading.textContent === '“정보” 단계')
      expect(screen.getByRole('checkbox', { name: '정보를 직접 입력하게 하기' })).toBeChecked()
    await user.click(screen.getByRole('button', { name: '다음' }))
  }
  const preview = within(screen.getByRole('region', { name: '구성 미리보기' }))
  await chooseOption(user, preview.getByRole('combobox', { name: /^인트로 디자인/ }), '매거진 커버')
  await chooseOption(user, preview.getByRole('combobox', { name: /^아웃트로 디자인/ }), '칩 줄')
  await user.click(preview.getByRole('checkbox', { name: '네온 사인' }))
  await user.click(screen.getByRole('button', { name: '다음' }))
  expect(
    calls.filter((call) => /^(Create|Update|Start|Generate|Render|SaveAuthoring)/.test(call)),
  ).toEqual([])
  await user.type(screen.getByLabelText('영상 템플릿 이름'), '내 카페 클립')
  await user.dblClick(screen.getByRole('button', { name: '저장' }))
  await waitFor(() => expect(writes).toHaveLength(1))
  await waitFor(() => expect(router.state.location.pathname).not.toBe('/video-templates/new'))
  expect(writes[0]).toMatchObject({
    name: '내 카페 클립',
    introPreset: 'cover',
    outroPreset: 'chips',
    allowedCaptionStyles: ['neon'],
  })
  const document = parseClipTemplate(writes[0].compositionBody)
  expect(document.stages).toHaveLength(7)
  expect(document.stages[0].name).toBe('처음 만난 카페')
  expect(document.stages[2].intent).toContain('음료·디저트')
  expect(document.fields[0]).toMatchObject({ label: '가격·위치', required: false })
  expect(calls).not.toContain('CreateAuthoringSession')
  expect(calls.filter((call) => /StartClip|RenderClip|PrepareClip/.test(call))).toEqual([])
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
  await screen.findByRole('heading', { name: '어떤 점을 바꿔 볼까요?' })
  expect(calls).toContain('GetAuthoringSession')
  expect(calls.filter((call) => /^(Create|Start)/.test(call))).toEqual([])
  expect(
    screen.queryByRole('heading', { name: '어떤 체험을 영상으로 남기나요?' }),
  ).not.toBeInTheDocument()
})

it('requests one clip heading batch and keeps manual stage names until an explicit choice', async () => {
  const starts: NonNullable<FakeTemplatesOptions['headingStarts']> = []
  const user = userEvent.setup()
  renderAppAt('/video-templates/new', {
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'p', modelId: 'w', stages: [Stage.WRITE] }],
      selections: [{ stage: Stage.WRITE, providerId: 'p', modelId: 'w' }],
    },
    templates: { headingStarts: starts },
    jobs: {
      jobs: [
        { id: 'heading-job', kind: 'template_heading_suggestion', stage: 'write', status: 'done' },
      ],
    },
  })
  await user.click(await screen.findByRole('button', { name: /^카페/ }))
  await user.click(await screen.findByRole('button', { name: /^디저트/ }))
  await user.click(screen.getByRole('button', { name: '다음' }))
  await user.click(screen.getByRole('button', { name: '다음' }))
  const field = screen.getByLabelText('훅 (첫 장면) 단계 이름')
  await user.clear(field)
  await user.type(field, '내 첫 장면')
  await user.dblClick(screen.getByRole('button', { name: '제목 제안 받기' }))
  await screen.findByRole('button', { name: '훅 (첫 장면) 후보 하나' })
  expect(starts).toHaveLength(1)
  expect(starts[0]).toMatchObject({
    targetKind: 'clip',
    targetPath: ['카페', '디저트·베이커리'],
    writeModel: { providerId: 'p', modelId: 'w' },
  })
  expect(starts[0].sections).toHaveLength(7)
  expect(field).toHaveValue('내 첫 장면')
  await user.click(screen.getByRole('button', { name: '훅 (첫 장면) 후보 둘' }))
  expect(field).toHaveValue('훅 (첫 장면) 후보 둘')
})
