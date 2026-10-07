import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { publishAuthoringDraft } from '@/test/authoring-ui'
import { parseClipComposition, type ClipRecipe } from '@/entities/clip-template'
import type { FakeClipsOptions } from '@/test/clips'
import type { FakeAuthoringOptions } from '@/test/authoring'
const body =
  '<clip version=\'1\'>\n<group id="menu" max="3"><field id="name" label="메뉴 이름"/></group>\n<stage name="외관">  keep &amp; spacing  </stage>\n<text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/></clip>'
const template = { id: 'owned', name: '장면 템플릿', compositionBody: body }
async function mount(
  user: ReturnType<typeof userEvent.setup>,
  clips: FakeClipsOptions = {},
  authoring: FakeAuthoringOptions = {},
  path = '/video-templates/owned',
) {
  const view = renderAppAt(path, {
    user: { id: 'alice' },
    clips: { templates: [template], ...clips },
    authoring,
  })
  await user.click(
    await screen.findByRole('button', {
      name: path.endsWith('/new') ? '직접 편집' : /직접 편집하기$/,
    }),
  )
  await screen.findByLabelText('템플릿 이름')
  return view
}
async function source(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('tab', { name: '원문' }))
  return screen.getByRole('textbox', { name: '원문' })
}
describe('durable clip template composition editing', () => {
  it('roundtrips group names/minimum and untouched source spans through builder/source/publication', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount(user, { writes })
    await user.click(screen.getByRole('button', { name: '항목 묶음 1' }))
    fireEvent.change(screen.getByLabelText('항목 묶음 이름'), { target: { value: '메뉴 & 음료' } })
    fireEvent.change(screen.getByLabelText('최소 항목 개수'), { target: { value: '2' } })
    const value = ((await source(user)) as HTMLTextAreaElement).value
    expect(parseClipComposition(value).groups).toMatchObject([
      { id: 'menu', label: '메뉴 & 음료', min: 2, max: 3 },
    ])
    expect(value).toContain('<stage name="외관">  keep &amp; spacing  </stage>')
    await user.click(screen.getByRole('tab', { name: '구성 편집' }))
    await user.click(screen.getByRole('button', { name: '메뉴 & 음료' }))
    expect(screen.getByLabelText('최소 항목 개수')).toHaveValue('2')
    await publishAuthoringDraft(user)
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionBody).toBe(value)
  })
  it('keeps exact raw bytes when only the name changes and performs no provider work', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    const calls: string[] = []
    await mount(user, { writes, calls })
    expect(await source(user)).toHaveValue(body)
    await user.type(screen.getByLabelText('템플릿 이름'), ' 새 이름')
    await publishAuthoringDraft(user)
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionBody).toBe(body)
    expect(calls).not.toContain('StartClipGeneration')
  })
  it('retains unfinished raw composition across AI/direct modes with publication blocked', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    await mount(user, {}, { patches })
    fireEvent.change(await source(user), { target: { value: '<clip version="1"><stage' } })
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
    await screen.findByRole('heading', { name: '어떤 점을 바꿔 볼까요?' })
    expect(patches[0].body).toBe('<clip version="1"><stage')
    await user.click(screen.getByRole('button', { name: /직접 편집하기$/ }))
    expect(await source(user)).toHaveValue('<clip version="1"><stage')
    expect(screen.queryByRole('button', { name: /변경사항 저장하기$/ })).not.toBeInTheDocument()
  })
  it('supports seed-free new direct creation and retains default design fields', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount(user, { templates: [], writes }, {}, '/video-templates/new')
    await user.type(screen.getByLabelText('템플릿 이름'), '새 구성')
    expect(await source(user)).toHaveValue('<clip version="1"/>')
    await publishAuthoringDraft(user)
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(await screen.findByText('“새 구성” 영상 구성을 새로 저장했어요.')).toBeInTheDocument()
    expect(writes[0]).toMatchObject({
      name: '새 구성',
      compositionBody: '<clip version="1"/>',
      introPreset: 'a',
      outroPreset: 'b',
      allowedCaptionStyles: [],
    })
  })
})
