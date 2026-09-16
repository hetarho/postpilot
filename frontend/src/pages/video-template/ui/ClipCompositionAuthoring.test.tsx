import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
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
  '<clip version=\'1\' intro="b" caption="bold" outro="e">\n  <field id="place" label="장소" required="false">어디인가요?</field>\n  <text id="badge" kind="fixed" role="badge" basis="output-start" start="0" end="3">  🌿 A &amp; B  </text>\n<text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
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
    expect(edited).toContain('<clip version=\'1\' intro="b" caption="bold" outro="e">')
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
    expect(screen.getByRole('alert')).toHaveTextContent('badge')
    expect(screen.getByRole('alert')).toHaveTextContent('순서')
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
    fireEvent.change(input, { target: { value: body.replace('end="3"', 'end="4"') } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
    expect(writes).toHaveLength(0)
  })
  it('keeps incomplete timing edits in controls and exposes the same invalid source', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('button', { name: '공개 문구' }))
    fireEvent.change(screen.getByRole('textbox', { name: '시작 (초)' }), { target: { value: '5' } })
    expect(screen.getByRole('textbox', { name: '시작 (초)' })).toHaveValue('5')
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    const pasted = ((await source()) as HTMLTextAreaElement).value
    expect(pasted).toContain('start="5"')
    expect(pasted).toContain('>  🌿 A &amp; B  </text>')
    expect(() => parseClipTemplate(pasted)).toThrow('invalid_interval')
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
  it('offers only the information, guide and badge a template may declare', async () => {
    const user = userEvent.setup()
    mount()
    await screen.findByRole('button', { name: '장소' })
    for (const name of ['정보 추가', '항목 묶음 추가', '구성 안내 추가', '화면 문구 추가'])
      expect(screen.getByRole('button', { name })).toBeInTheDocument()
    for (const name of ['장면 추가', '반복 추가'])
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
    // Caption pace and the accent belong to the project now (CLIP-139).
    expect(screen.queryByRole('combobox', { name: /강조색/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: /자막 속도/ })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '화면 문구 추가' }))
    const added = parseClipTemplate(((await source()) as HTMLTextAreaElement).value)
    expect(added.elements.map((e) => e.role)).toEqual(['badge', 'hook', 'ending', 'badge'])
    expect(added.sections).toEqual([])
  })
  it('a text carries no caption or information role and no cut timing', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('button', { name: '공개 문구' }))
    expect(screen.queryByRole('combobox', { name: /^문구 용도/ })).not.toBeInTheDocument()
    await user.click(screen.getByRole('combobox', { name: /표시 구간 기준/ }))
    expect(screen.getByRole('option', { name: '영상 전체' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: '해당 컷 안에서' })).not.toBeInTheDocument()
  })
  it('says a legacy template kept its scenes as guidance and clears the notice on save', async () => {
    const user = userEvent.setup(),
      writes: ClipRecipe[] = []
    const converted =
      '<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="장소"/><guide>가장 이른 클립으로 시작</guide><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
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
  // What the server hands the editor for that template: its scenes carried into
  // the guide, its ending still departing from the preset (CLIP-114, CLIP-140).
  const corpus = JSON.parse(
    readFileSync(
      resolve(
        import.meta.dirname,
        '../../../../../backend/internal/clip/composition/testdata/corpus.json',
      ),
      'utf8',
    ),
  ) as { cases: { name: string; converted?: string }[] }
  const legacy = corpus.cases.find(
    (c) => c.name === 'restaurant v2 legacy template converts for the editor',
  )!.converted!
  mount({
    calls,
    templates: [
      {
        ...template,
        name: '맛집 테스트',
        compositionBody: legacy,
        compositionLegacy: true,
        compositionConverted: true,
      },
    ],
  })
  expect(await screen.findByRole('tab', { name: 'B 위아래 가로선' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  expect(screen.getByRole('tab', { name: 'E 점수 강조' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByText(/구성 안내로 옮겼어요/)).toBeInTheDocument()
  expect(screen.getByRole('alert')).toHaveTextContent('인트로·아웃트로 구조')
  expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: '프리셋 형태로 다시 만들기' }))
  const rebuilt = ((await source()) as HTMLTextAreaElement).value
  const doc = parseClipTemplate(rebuilt),
    ending = doc.elements.find((e) => e.id === 'closing_verdict')!
  expect(ending.basis).toBe('output-end')
  expect(ending.rows[0].kind).toBe('ai')
  expect(ending.rows[0].parts.map((p) => p.literal).join('')).toContain(
    '새로운 추천이나 인사말을 덧붙이지 마세요.',
  )
  expect(doc.sections).toEqual([])
  expect(doc.guidance.join('\n')).toContain('arrival')
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(calls.some((c) => /Seed|Quote|Start|Generate|CreateVideo|UpdateVideo/.test(c))).toBe(false)
})
