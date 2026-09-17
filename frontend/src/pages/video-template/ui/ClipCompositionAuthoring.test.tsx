import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import {
  CLIP_COMPOSITION_EXAMPLE,
  parseClipComposition,
  parseClipTemplate,
  type ClipRecipe,
} from '@/entities/clip-template'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import type { FakeClipsOptions } from '@/test/clips'

const body =
  '<clip version=\'1\'>\n  <field id="place" label="장소" required="false">어디인가요?</field>\n  <text id="badge" kind="fixed" role="badge" position="top">  🌿 A &amp; B  </text>\n<text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/></clip>'
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
      '<clip version=\'1\'>\n<group id="menu" max="3"><field id="name" label="메뉴 이름"/></group>\n<guide>  keep &amp; spacing  </guide>\n<text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/></clip>'
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
    expect(edited).toContain("<clip version='1'>")
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
    // A template declares no timing at all (CLIP-66), and the refusal names the
    // entry that declared one.
    const invalid = body.replace('role="badge"', 'role="badge" basis="whole"')
    fireEvent.change(input, { target: { value: invalid } })
    expect(screen.getByRole('alert')).toHaveTextContent('badge')
    expect(screen.getByRole('alert')).toHaveTextContent('표시 시간을 적지 않아요')
    expect(input).toHaveValue(invalid)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    // A construct the template grammar no longer holds says so by name.
    fireEvent.change(input, {
      target: {
        value: body.replace(
          '<text id="badge"',
          '<scene id="footage" scope="scene"/><text id="badge"',
        ),
      },
    })
    expect(screen.getByRole('alert')).toHaveTextContent('장면(scene·repeat)은 더 이상')
    fireEvent.change(input, { target: { value: body.replace('id="badge"', 'id="badge2"') } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
    expect(writes).toHaveLength(0)
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
      { id: 'menu', label: '메뉴', min: 1, max: CLIP_COMPOSITION_LIMITS.items },
    ])
    expect(guide).toContain('group(id, label?, min?, max?)')
    expect(calls.some((c) => /Seed|Quote|Start|Generate|CreateVideo|UpdateVideo/.test(c))).toBe(
      false,
    )
  })
  it('creates a template with one save and no design anywhere on the screen', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    const { router } = mount({ writes }, '/video-templates/new')
    await user.type(await screen.findByLabelText('템플릿 이름'), '새 구성')
    expect(screen.queryByLabelText(/스타일|style/i)).not.toBeInTheDocument()
    // The design belongs to the clip, so the editor offers none of it (CLIP-42).
    for (const name of ['B 위아래 가로선', '크게 강조', 'E 점수 강조'])
      expect(screen.queryByRole('tab', { name })).not.toBeInTheDocument()
    // 원문 and the builder are there from the first keystroke, over an empty body.
    expect(screen.getByRole('tab', { name: '원문' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '인트로 문구 추가' }))
    expect(
      parseClipTemplate(((await source()) as HTMLTextAreaElement).value).elements.map((e) => [
        e.role,
        e.rows.length,
      ]),
    ).toEqual([['hook', 1]])
    await user.dblClick(screen.getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/video-templates/video-template-1'),
    )
    expect(writes).toHaveLength(1)
    expect(writes[0]).not.toHaveProperty('copyStyles')
    // A saved body names no design at all: the presets are the project's
    // (CLIP-14, CLIP-139).
    expect(parseClipComposition(writes[0].compositionBody!).root.attributes).toEqual({
      version: '1',
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
  it('offers only the information, guide and badge a template may declare', async () => {
    const user = userEvent.setup()
    mount()
    await screen.findByRole('button', { name: '장소' })
    for (const name of [
      '정보 추가',
      '항목 묶음 추가',
      '구성 안내 추가',
      '구성 단계 추가',
      '인트로 문구 추가',
      '자막 추가',
      '아웃트로 문구 추가',
      '표시 문구 추가',
    ])
      expect(screen.getByRole('button', { name })).toBeInTheDocument()
    for (const name of ['장면 추가', '반복 추가'])
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
    // Caption pace and the accent belong to the project now (CLIP-139).
    expect(screen.queryByRole('combobox', { name: /강조색/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: /자막 속도/ })).not.toBeInTheDocument()
    // Each kind lands at the end of the outline, in the order it was added.
    await user.click(screen.getByRole('button', { name: '자막 추가' }))
    await user.click(screen.getByRole('button', { name: '표시 문구 추가' }))
    const added = parseClipTemplate(((await source()) as HTMLTextAreaElement).value)
    expect(added.outline.map((entry) => added.elements[entry.index].role)).toEqual([
      'badge',
      'hook',
      'ending',
      'caption',
      'badge',
    ])
    expect(added.sections).toEqual([])
  })
  it('reorders and deletes every kind of entry, regions included', async () => {
    const user = userEvent.setup()
    mount()
    // The intro entry is an outline row like any other now (CLIP-112): it can be
    // moved and removed, and the body's order is what the list shows.
    const before = parseClipTemplate(((await source()) as HTMLTextAreaElement).value)
    expect(before.outline.map((e) => before.elements[e.index].role)).toEqual([
      'badge',
      'hook',
      'ending',
    ])
    await user.click(await screen.findByRole('tab', { name: '구성 편집' }))
    const rows = screen.getAllByRole('button', { name: '위로 이동' })
    await user.click(rows[rows.length - 1])
    const moved = parseClipTemplate(((await source()) as HTMLTextAreaElement).value)
    expect(moved.outline.map((e) => moved.elements[e.index].role)).toEqual([
      'badge',
      'ending',
      'hook',
    ])
    await user.click(await screen.findByRole('tab', { name: '구성 편집' }))
    await user.click(screen.getAllByRole('button', { name: '아웃트로' })[0])
    await user.click(screen.getByRole('button', { name: '아웃트로 삭제' }))
    const removed = parseClipTemplate(((await source()) as HTMLTextAreaElement).value)
    expect(removed.outline.map((e) => removed.elements[e.index].role)).toEqual(['badge', 'hook'])
  })

  it('a text carries no role of its own and no timing at all', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('button', { name: '공개 문구' }))
    expect(screen.queryByRole('combobox', { name: /^문구 용도/ })).not.toBeInTheDocument()
    // Where an entry stands in the outline is the only position it declares
    // (CLIP-112): no basis, no start and no end (CLIP-66).
    expect(screen.queryByRole('combobox', { name: /표시 구간 기준/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: '시작 (초)' })).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: '끝 (초)' })).not.toBeInTheDocument()
  })
  it('says a legacy template kept its scenes as guidance and clears the notice on save', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    const converted =
      '<clip version="1"><field id="place" label="장소"/><guide>가장 이른 클립으로 시작</guide><text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/></clip>'
    mount({
      templates: [{ ...template, compositionBody: converted, compositionConverted: true }],
      writes,
    })
    expect(await screen.findByText(/구성 안내로 옮겼어요/)).toBeInTheDocument()
    expect(await source()).toHaveValue(converted)
    await user.click(screen.getByRole('tab', { name: '구성 편집' }))
    await user.click(screen.getByRole('button', { name: '장소' }))
    fireEvent.change(screen.getByRole('textbox', { name: '정보 이름' }), {
      target: { value: '촬영 장소' },
    })
    await waitFor(() => expect(screen.queryByText(/구성 안내로 옮겼어요/)).not.toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(parseClipTemplate(writes[0].compositionBody!).guidance).toEqual([
      '가장 이른 클립으로 시작',
    ])
  })
})

it('accepts an outro entry holding more lines than a preset draws', async () => {
  // How many of them are drawn is the project's preset to decide, and the
  // surplus is a CLIP-108 notice at render rather than a refusal here.
  mount({
    templates: [
      {
        ...template,
        compositionBody: body.replace(
          '<text id="outro" kind="fixed" role="ending"/>',
          '<text id="outro" kind="fixed" role="ending"><row>첫 줄</row><row>둘째 줄</row><row>남겨 둘 문구</row></text>',
        ),
      },
    ],
  })
  const saved = parseClipTemplate(((await source()) as HTMLTextAreaElement).value)
  expect(saved.elements.find((e) => e.role === 'ending')!.rows).toHaveLength(3)
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
