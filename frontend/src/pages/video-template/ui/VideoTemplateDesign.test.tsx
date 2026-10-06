import { describe, expect, it } from 'vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { chooseOption } from '@/test/listbox'
import type { ClipRecipe } from '@/entities/clip-template'
import type { FakeClipsOptions } from '@/test/clips'

const body =
  '<clip version="1"><field id="place" label="장소">어디인가요?</field>' +
  '<text id="hook" kind="fixed" role="hook"/><text id="ending" kind="fixed" role="ending"/></clip>'
const template = {
  id: 'owned',
  name: '장면 템플릿',
  compositionBody: body,
  introPreset: 'cover' as const,
  outroPreset: 'e' as const,
  allowedCaptionStyles: ['neon'],
}
const mount = async (clips: FakeClipsOptions = {}, path = '/video-templates/owned') => {
  const rendered = renderAppAt(path, {
    user: { id: 'alice' },
    clips: { templates: [template], ...clips },
  })
  await userEvent.setup().click(await screen.findByRole('button', { name: '직접 편집' }))
  return rendered
}
const preview = () => within(screen.getByRole('region', { name: '구성 미리보기' }))
const save = () => screen.getByRole('button', { name: '저장' })

// CLIP-166: the template's preview is where its starting design is chosen, and the docked 저장
// saves it with the name and the body.
describe('a video template’s design selection', () => {
  it('opens in the stored selection, edits the draft from the preview and saves it all', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount({ writes })
    await screen.findByRole('region', { name: '구성 미리보기' })
    const intro = preview().getByRole('combobox', { name: /^인트로 디자인/ })
    expect(intro).toHaveAccessibleName('인트로 디자인 매거진 커버')
    expect(preview().getByRole('combobox', { name: /^아웃트로 디자인/ })).toHaveAccessibleName(
      '아웃트로 디자인 E 점수 강조',
    )
    const styles = preview().getByRole('group', { name: '자막 스타일' })
    expect(within(styles).getByRole('checkbox', { name: '네온 사인' })).toBeChecked()
    expect(save()).toBeDisabled()

    // A choice and its undoing: the selection is part of what is dirty.
    await chooseOption(user, intro, '명조 매거진')
    expect(save()).toBeEnabled()
    await chooseOption(
      user,
      preview().getByRole('combobox', { name: /^인트로 디자인/ }),
      '매거진 커버',
    )
    expect(save()).toBeDisabled()

    await chooseOption(user, preview().getByRole('combobox', { name: /^아웃트로 디자인/ }), '칩 줄')
    await user.click(within(styles).getByRole('checkbox', { name: '워드 팝' }))
    await user.click(within(styles).getByRole('checkbox', { name: '네온 사인' }))
    await user.click(within(styles).getByRole('checkbox', { name: '크게 강조' }))
    await user.click(save())
    await screen.findByText('저장했어요')
    expect(writes).toHaveLength(1)
    // The catalogue's order, whatever order they were ticked in.
    expect(writes[0]).toMatchObject({
      name: '장면 템플릿',
      compositionBody: body,
      introPreset: 'cover',
      outroPreset: 'chips',
      allowedCaptionStyles: ['bold', 'word-pop'],
    })
    expect(save()).toBeDisabled()
  })

  it('saves a selection of no styles as one', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount({ writes })
    await screen.findByRole('region', { name: '구성 미리보기' })
    const styles = preview().getByRole('group', { name: '자막 스타일' })
    await user.click(within(styles).getByRole('checkbox', { name: '네온 사인' }))
    await user.click(save())
    await screen.findByText('저장했어요')
    expect(writes[0].allowedCaptionStyles).toEqual([])
  })

  // CLIP-167: the source is the outline alone — it neither shows the selection nor changes it.
  it('leaves the selection out of the source and a pasted source leaves it alone', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount({ writes })
    await user.click(await screen.findByRole('tab', { name: '원문' }))
    const source = screen.getByRole('textbox', { name: '원문' })
    expect((source as HTMLTextAreaElement).value).not.toMatch(/cover|neon/)
    const pasted = body.replace('장소', '가게')
    fireEvent.change(source, { target: { value: pasted } })
    await user.click(save())
    await screen.findByText('저장했어요')
    expect(writes[0]).toMatchObject({
      compositionBody: pasted,
      introPreset: 'cover',
      outroPreset: 'e',
      allowedCaptionStyles: ['neon'],
    })
  })

  it('starts a new template at a new project’s design and creates it with the choice', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount({ templates: [], writes }, '/video-templates/new')
    await user.type(await screen.findByLabelText('템플릿 이름'), '새 템플릿')
    const intro = preview().getByRole('combobox', { name: /^인트로 디자인/ })
    expect(intro).toHaveAccessibleName('인트로 디자인 A 크기만')
    await chooseOption(user, intro, '로어서드')
    await user.click(save())
    await screen.findByRole('heading', { name: '새 템플릿', level: 1 })
    expect(writes[0]).toMatchObject({
      introPreset: 'lower',
      outroPreset: 'b',
      allowedCaptionStyles: [],
    })
  })
})
