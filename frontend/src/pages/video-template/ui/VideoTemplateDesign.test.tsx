import { describe, expect, it } from 'vitest'
import { fireEvent, screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { publishAuthoringDraft } from '@/test/authoring-ui'
import { chooseOption } from '@/test/listbox'
import type { ClipRecipe } from '@/entities/clip-template'
const body =
  '<clip version="1"><field id="place" label="장소">어디인가요?</field><text id="hook" kind="fixed" role="hook"/><text id="ending" kind="fixed" role="ending"/></clip>'
const template = {
  id: 'owned',
  name: '장면 템플릿',
  compositionBody: body,
  introPreset: 'cover' as const,
  outroPreset: 'e' as const,
  allowedCaptionStyles: ['neon'],
}
function mount(writes: ClipRecipe[] = []) {
  return renderAppAt('/video-templates/owned', {
    user: { id: 'alice' },
    clips: { templates: [template], writes },
  })
}
describe('separate retained video template design', () => {
  it('opens the saved reading on its one action row, then the example of its stored design', async () => {
    mount()
    const example = await screen.findByText('구성 예시')
    expect(
      screen.getByText('인트로 매거진 커버 · 자막 네온 사인 · 아웃트로 점수 강조'),
    ).toBeInTheDocument()
    const edit = screen.getByRole('button', { name: /시작 디자인 편집$/ })
    expect(edit).toHaveTextContent('시작 디자인')
    // The actions come first, the composition example and design after them (CLIP-42).
    expect(edit.compareDocumentPosition(example) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    // No 확인하기 step title and no saved-and-usable or design-help lines.
    expect(screen.queryByRole('heading', { name: /확인하기$/ })).not.toBeInTheDocument()
    expect(screen.queryByText(/사용할 수 있어요/)).not.toBeInTheDocument()
    expect(screen.queryByText(/인트로·아웃트로와 자막 스타일을 바꿔요/)).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: /^인트로 디자인/ })).not.toBeInTheDocument()
  })
  it('shows saved design editing beside named text methods and exposes no competing name/body draft', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('button', { name: /시작 디자인 편집$/ }))
    const preview = within(await screen.findByRole('region', { name: '구성 미리보기' }))
    expect(preview.getByRole('combobox', { name: /^인트로 디자인/ })).toHaveAccessibleName(
      '인트로 디자인 매거진 커버',
    )
    expect(preview.getByRole('combobox', { name: /^아웃트로 디자인/ })).toHaveAccessibleName(
      '아웃트로 디자인 점수 강조',
    )
    expect(screen.queryByLabelText('템플릿 이름')).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '원문' })).not.toBeInTheDocument()
  })
  it('saves design defaults and an explicit empty caption choice without changing composition', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    mount(writes)
    await user.click(await screen.findByRole('button', { name: /시작 디자인 편집$/ }))
    const preview = within(await screen.findByRole('region', { name: '구성 미리보기' }))
    await chooseOption(user, preview.getByRole('combobox', { name: /^아웃트로 디자인/ }), '칩 줄')
    await user.click(preview.getByRole('checkbox', { name: '네온 사인' }))
    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0]).toMatchObject({
      name: template.name,
      compositionBody: body,
      introPreset: 'cover',
      outroPreset: 'chips',
      allowedCaptionStyles: [],
    })
  })
  it('shared text publication preserves starting design without copying it into the raw source', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    mount(writes)
    await user.click(await screen.findByRole('button', { name: /직접 편집하기$/ }))
    await screen.findByLabelText('템플릿 이름')
    if (screen.getByRole('button', { name: '고급' }).getAttribute('aria-expanded') !== 'true')
      await user.click(screen.getByRole('button', { name: '고급' }))
    await user.click(screen.getByRole('tab', { name: '원문' }))
    const source = screen.getByRole('textbox', { name: '원문' })
    expect(source).toHaveValue(body)
    fireEvent.change(source, { target: { value: body.replace('장소', '가게') } })
    await publishAuthoringDraft(user)
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0]).toMatchObject({
      compositionBody: body.replace('장소', '가게'),
      introPreset: 'cover',
      outroPreset: 'e',
      allowedCaptionStyles: ['neon'],
    })
  })
  it('warns before leaving unsaved design changes and keeps them when dismissal is chosen', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    mount(writes)
    await user.click(await screen.findByRole('button', { name: /시작 디자인 편집$/ }))
    const preview = within(await screen.findByRole('region', { name: '구성 미리보기' }))
    await user.click(preview.getByRole('checkbox', { name: '네온 사인' }))
    await user.click(screen.getByRole('button', { name: /저장된 영상 템플릿.*보기$/ }))
    expect(await screen.findByRole('dialog')).toHaveTextContent(
      '저장하지 않은 영상 템플릿 변경사항이 사라져요.',
    )
    await user.keyboard('{Escape}')
    expect(preview.getByRole('checkbox', { name: '네온 사인' })).not.toBeChecked()
    expect(writes).toHaveLength(0)
  })
})
