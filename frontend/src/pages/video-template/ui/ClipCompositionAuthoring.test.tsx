import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import {
  CLIP_COMPOSITION_EXAMPLE,
  parseClipComposition,
  type ClipRecipe,
} from '@/entities/clip-template'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import type { FakeClipsOptions } from '@/test/clips'

const body =
  '<clip version=\'1\' intro="b" caption="bold" outro="e" pace=\'steady\'>\n  <field id="place" label="장소" required="false">어디인가요?</field>\n  <scene id="scene" scope="scene"/>\n  <text id="caption" kind="fixed" role="caption" basis="output-start" start="0" end="3">  🌿 A &amp; B  </text>\n<text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
const template = {
  id: 'owned',
  name: '장면 템플릿',
  compositionBody: body,
  compositionLegacy: false,
  informationFields: [],
  cutGuidance: '',

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
  it('round-trips the group name and minimum through the builder, source and save', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    const original =
      '<clip version=\'1\' intro="b" caption="bold" outro="e">\n<group id="menu" max="3"><field id="name" label="메뉴 이름"/></group>\n<guide>  keep &amp; spacing  </guide>\n<text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
    mount({ templates: [{ ...template, compositionBody: original }], writes })
    await user.click(await screen.findByRole('button', { name: '항목 묶음 1' }))
    expect(screen.getByLabelText('항목 묶음 이름')).toHaveValue('')
    expect(screen.getByLabelText('최소 항목 개수')).toHaveValue('0')
    fireEvent.change(screen.getByLabelText('항목 묶음 이름'), { target: { value: '메뉴 & 음료' } })
    fireEvent.change(screen.getByLabelText('최소 항목 개수'), { target: { value: '2' } })
    const edited = ((await source()) as HTMLTextAreaElement).value
    expect(parseClipComposition(edited).groups).toMatchObject([
      { id: 'menu', label: '메뉴 & 음료', min: 2, max: 3 },
    ])
    expect(edited).toContain('<guide>  keep &amp; spacing  </guide>')
    await user.click(screen.getByRole('tab', { name: '구성 편집' }))
    await user.click(screen.getByRole('button', { name: '메뉴 & 음료' }))
    expect(screen.getByLabelText('항목 묶음 이름')).toHaveValue('메뉴 & 음료')
    expect(screen.getByLabelText('최소 항목 개수')).toHaveValue('2')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(writes).toHaveLength(1)
    expect(writes[0]).not.toHaveProperty('copyStyles')
    expect(writes[0].compositionBody).toBe(edited)
  })

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
    expect(edited).toContain(
      '<clip version=\'1\' intro="b" caption="bold" outro="e" pace=\'steady\'>',
    )
    expect(edited).toContain('>  🌿 A &amp; B  </text>')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(writes).toHaveLength(1)
    expect(writes[0]).not.toHaveProperty('copyStyles')
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
    expect(parseClipComposition(CLIP_COMPOSITION_EXAMPLE).groups).toMatchObject([
      { id: 'menu', label: '', min: 0, max: CLIP_COMPOSITION_LIMITS.items },
    ])
    expect(guide).toContain('group(id, label?, min?, max?)')
    expect(calls.some((c) => /Seed|Quote|Start|Generate|CreateVideo|UpdateVideo/.test(c))).toBe(
      false,
    )
  })
  it('creates a native template with one save and the caption treatment', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    const { router } = mount({ writes }, '/video-templates/new')
    await user.type(await screen.findByLabelText('템플릿 이름'), '새 구성')
    expect(screen.queryByLabelText(/스타일|style/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '구성 편집' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.click(screen.getByRole('tab', { name: 'B 위아래 가로선' }))
    await user.click(screen.getByRole('tab', { name: '크게 강조' }))
    expect(screen.queryByRole('tab', { name: '원문' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: 'E 점수 강조' }))
    expect(
      parseClipComposition(((await source()) as HTMLTextAreaElement).value).elements.map((e) => [
        e.role,
        e.rows.length,
      ]),
    ).toEqual([
      ['hook', 2],
      ['ending', 3],
    ])
    await user.dblClick(screen.getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/video-templates/video-template-1'),
    )
    expect(writes).toHaveLength(1)
    expect(writes[0]).not.toHaveProperty('copyStyles')
    expect(parseClipComposition(writes[0].compositionBody!).design).toEqual({
      intro: 'b',
      caption: 'bold',
      outro: 'e',
    })
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
  it('edits repeated item scenes and text rows through supported controls', async () => {
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
    await user.click(screen.getByRole('button', { name: '아웃트로' }))
    expect(screen.queryByRole('combobox', { name: /^화면 위치/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: /^문구 용도/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '텍스트 행 추가' })).not.toBeInTheDocument()
    const slot = within(screen.getByRole('region', { name: '슬롯 1' }))
    fireEvent.change(slot.getByRole('textbox', { name: '문구 1' }), {
      target: { value: '다음 이야기' },
    })
    fireEvent.change(screen.getByRole('textbox', { name: '시작 (초)' }), {
      target: { value: '-4' },
    })
    const changed = parseClipComposition(((await source()) as HTMLTextAreaElement).value)
    const ending = changed.elements.find((element) => element.id === 'closing')!
    expect(ending.rows).toHaveLength(3)
    expect(ending.basis).toBe('output-end')
    expect(ending.startMs).toBe(-4000)
  })
})

it('keeps a populated excess outro slot visible until the owner resolves it', async () => {
  const user = userEvent.setup()
  mount({
    templates: [
      {
        ...template,
        compositionBody: body.replace(
          '<text id="outro" kind="fixed" role="ending" basis="output-end"/>',
          '<text id="outro" kind="fixed" role="ending" basis="output-end"><row>첫 줄</row><row>둘째 줄</row><row>남겨 둘 문구</row></text>',
        ),
      },
    ],
  })
  await user.click(await screen.findByRole('tab', { name: 'B 가로선 구분' }))
  expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: '아웃트로' }))
  const third = within(screen.getByRole('region', { name: '슬롯 3' }))
  expect(third.getByRole('textbox', { name: '문구 1' })).toHaveValue('남겨 둘 문구')
  expect(third.getByRole('status')).toHaveTextContent('담을 수 없는 줄')
  const second = within(screen.getByRole('region', { name: '슬롯 2' }))
  fireEvent.change(second.getByRole('textbox', { name: '문구 1' }), {
    target: { value: '둘째 줄 남겨 둘 문구' },
  })
  fireEvent.change(third.getByRole('textbox', { name: '문구 1' }), { target: { value: '' } })
  await user.click(screen.getAllByRole('button', { name: '프리셋 형태로 다시 만들기' })[0])
  const document = parseClipComposition(((await source()) as HTMLTextAreaElement).value)
  expect(document.design.outro).toBe('b')
  expect(
    document.elements
      .find((e) => e.role === 'ending')!
      .rows.map((r) => r.parts.map((p) => p.literal).join('')),
  ).toEqual(['첫 줄', '둘째 줄 남겨 둘 문구'])
  expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
})

it('explicitly rebuilds the restaurant legacy ending without losing its row text', async () => {
  const user = userEvent.setup(),
    calls: string[] = []
  const legacy = readFileSync(
    resolve(
      import.meta.dirname,
      '../../../../../backend/internal/platform/db/testdata/restaurant-v2-before-design-selection.xml',
    ),
    'utf8',
  )
    .replace(/ styles="[^"]*"| style="[^"]*"/g, '')
    .replace('version="1"', 'version="1" intro="b" caption="bold" outro="e"')
  mount({
    calls,
    templates: [
      { ...template, name: '맛집 테스트', compositionBody: legacy, compositionLegacy: true },
    ],
  })
  expect(await screen.findByRole('tab', { name: 'B 위아래 가로선' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  expect(screen.getByRole('tab', { name: 'E 점수 강조' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('alert')).toHaveTextContent('인트로·아웃트로 구조')
  expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: '프리셋 형태로 다시 만들기' }))
  const rebuilt = ((await source()) as HTMLTextAreaElement).value
  const doc = parseClipComposition(rebuilt),
    ending = doc.elements.find((e) => e.id === 'closing_verdict')!
  expect(ending.basis).toBe('output-end')
  expect(ending.rows[0].kind).toBe('ai')
  expect(ending.rows[0].parts.map((p) => p.literal).join('')).toContain(
    '새로운 추천이나 인사말을 덧붙이지 마세요.',
  )
  expect(doc.root.children.some((n) => n.attributes.id === ending.id)).toBe(true)
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(calls.some((c) => /Seed|Quote|Start|Generate|CreateVideo|UpdateVideo/.test(c))).toBe(false)
})
