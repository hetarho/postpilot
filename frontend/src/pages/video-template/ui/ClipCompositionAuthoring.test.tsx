import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import {
  CLIP_COMPOSITION_EXAMPLE,
  parseClipComposition,
  type ClipRecipe,
} from '@/entities/clip-template'
import type { FakeClipsOptions } from '@/test/clips'

const body = `<clip version='1' styles='clean memo' pace='steady'>
  <field id="place" label="장소" required="false">어디인가요?</field>
  <scene id="scene" scope="scene"/>
  <text id="caption" kind="fixed" role="caption" basis="output-start" start="0" end="3">  🌿 A &amp; B  </text>
</clip>`
const template = {
  id: 'owned',
  name: '장면 템플릿',
  compositionBody: body,
  compositionLegacy: false,
  informationFields: [],
  cutGuidance: '',
  copyStyles: ['clean'] as ClipRecipe['copyStyles'],
  accent: '' as const,
  preset: '' as const,
}
const mount = (clips: FakeClipsOptions = {}, path = '/video-templates/owned') =>
  renderAppAt(path, { user: { id: 'alice' }, clips: { templates: [template], ...clips } })
const source = async () => {
  await userEvent.click(await screen.findByRole('tab', { name: '원문' }))
  return screen.getByRole('textbox', { name: '원문' })
}

describe('composition template authoring', () => {
  it('keeps untouched source byte-exact across keyboard mode switches and never saves on open', async () => {
    const calls: string[] = []
    mount({ calls })
    expect(await source()).toHaveValue(body)
    const sourceTab = screen.getByRole('tab', { name: '원문' })
    sourceTab.focus()
    await userEvent.keyboard('{ArrowLeft}')
    expect(screen.getByRole('tab', { name: '구성 편집' })).toHaveFocus()
    await userEvent.keyboard('{ArrowRight}')
    expect(screen.getByRole('textbox', { name: '원문' })).toHaveValue(body)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(calls).not.toContain('UpdateVideoTemplate')
    expect(screen.queryByRole('combobox', { name: /프리셋/ })).not.toBeInTheDocument()
  })
  it('renames a field without changing identity or unrelated literal bytes', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    mount({ writes })
    await user.click(await screen.findByRole('button', { name: '장소' }))
    const label = screen.getByRole('textbox', { name: '정보 이름' })
    await user.clear(label)
    await user.type(label, '촬영 장소')
    await user.click(screen.getByRole('checkbox', { name: '필수로 받기' }))
    const edited = ((await source()) as HTMLTextAreaElement).value
    const doc = parseClipComposition(edited)
    expect(doc.fields[0]).toMatchObject({ id: 'place', label: '촬영 장소', required: true })
    expect(edited).toContain(`<clip version='1' styles='clean memo' pace='steady'>`)
    expect(edited).toContain('>  🌿 A &amp; B  </text>')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(writes).toHaveLength(1)
    expect(writes[0].compositionBody).toBe(edited)
  })
  it('preserves malformed pasted source, rejects save and recovers after correction', async () => {
    const writes: ClipRecipe[] = []
    mount({ writes })
    const input = await source()
    const invalid = body.replace('end="3"', 'end="-1"')
    fireEvent.change(input, { target: { value: invalid } })
    expect(screen.getByRole('alert')).toHaveTextContent('caption')
    expect(screen.getByRole('alert')).toHaveTextContent('순서')
    expect(input).toHaveValue(invalid)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    fireEvent.change(input, { target: { value: body.replace('end="3"', 'end="4"') } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
    expect(writes).toHaveLength(0)
  })
  it('keeps incomplete timing edits in controls and exposes the same invalid source', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('button', { name: '자막' }))
    fireEvent.change(screen.getByRole('textbox', { name: '시작 (초)' }), { target: { value: '5' } })
    expect(screen.getByRole('textbox', { name: '시작 (초)' })).toHaveValue('5')
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    const pasted = ((await source()) as HTMLTextAreaElement).value
    expect(pasted).toContain('start="5"')
    expect(pasted).toContain('>  🌿 A &amp; B  </text>')
    expect(() => parseClipComposition(pasted)).toThrow('invalid_interval')
  })
  it('copies exact source and a parseable external-AI guide without paid calls', async () => {
    const user = userEvent.setup(),
      calls: string[] = []
    mount({ calls })
    await user.click(await screen.findByRole('button', { name: '원문 복사' }))
    expect(await navigator.clipboard.readText()).toBe(body)
    await user.click(screen.getByRole('button', { name: '형식 안내 복사' }))
    const guide = await navigator.clipboard.readText()
    expect(guide).toContain(CLIP_COMPOSITION_EXAMPLE)
    expect(guide).toContain('sourceChars=16000')
    expect(parseClipComposition(CLIP_COMPOSITION_EXAMPLE).groups).toEqual(['menu'])
    expect(calls.some((c) => /Seed|Quote|Start|Generate|CreateVideo|UpdateVideo/.test(c))).toBe(
      false,
    )
  })
  it('creates a native template with one save and no preset, allowing any approved style set', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    const { router } = mount({ writes }, '/video-templates/new')
    await user.type(await screen.findByLabelText('템플릿 이름'), '새 구성')
    await user.click(screen.getByRole('checkbox', { name: '메모' }))
    await user.click(screen.getByRole('checkbox', { name: '깔끔하게' }))
    expect(screen.getByRole('checkbox', { name: '깔끔하게' })).not.toBeDisabled()
    await user.dblClick(screen.getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/video-templates/video-template-1'),
    )
    expect(writes).toHaveLength(1)
    expect(parseClipComposition(writes[0].compositionBody!).styles).toEqual(['memo'])
  })
  it('opens converted content without writing and reports unavailable generation capability', async () => {
    const calls: string[] = []
    mount({
      templates: [{ ...template, compositionLegacy: true }],
      compositionPlanVersion: 4,
      calls,
    })
    expect(await screen.findByText(/이전 템플릿의 내용이/)).toBeInTheDocument()
    expect(await screen.findByText(/서버에서 아직/)).toBeInTheDocument()
    expect(await source()).toHaveValue(body)
    expect(calls).not.toContain('UpdateVideoTemplate')
  })
  it('edits repeated item scenes and card rows through supported controls', async () => {
    const user = userEvent.setup()
    mount({ templates: [{ ...template, compositionBody: CLIP_COMPOSITION_EXAMPLE }] })
    await user.click(await screen.findByRole('button', { name: /반복 구성/ }))
    await user.click(screen.getAllByRole('button', { name: '장면 추가' })[0])
    const repeated = parseClipComposition(((await source()) as HTMLTextAreaElement).value)
    expect(repeated.sections).toHaveLength(2)
    expect(
      repeated.sections.every((section) => section.scope === 'item' && section.repeat === 'menu'),
    ).toBe(true)
    await user.click(screen.getByRole('tab', { name: '구성 편집' }))
    await user.click(screen.getByRole('button', { name: '엔딩 카드' }))
    await user.click(screen.getByRole('combobox', { name: /^화면 위치/ }))
    await user.click(screen.getByRole('option', { name: '아래쪽' }))
    await user.click(screen.getByRole('button', { name: '카드 행 추가' }))
    const changed = parseClipComposition(((await source()) as HTMLTextAreaElement).value)
    const ending = changed.elements.find((element) => element.id === 'closing')!
    expect(ending.position).toBe('bottom')
    expect(ending.rows).toHaveLength(3)
    expect(ending.basis).toBe('output-end')
    expect(ending.startMs).toBe(-3000)
  })
})
