import { afterEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { CLIP_COMPOSITION_EXAMPLE } from '@/entities/clip-template'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const template = {
  id: 'menu',
  name: '여러 메뉴',
  compositionBody: CLIP_COMPOSITION_EXAMPLE,
  compositionLegacy: false,
  informationFields: [],
  cutGuidance: '',

  accent: '' as const,
  preset: '' as const,
}
afterEach(() => discardClipDraftQueues())

describe('template-defined project inputs', () => {
  it('takes footage for a project whose template only came back converted', async () => {
    // The server rewrites a template still written under the old grammar into the current one on
    // the way out and flags it, while the STORED body waits for the owner's own save (CLIP-140).
    // The project froze that stored body, so the two differ although nobody edited the template.
    const stored =
      '<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="상호명" required="true"/><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
    const converted = stored.replace('<field', '<guide>장면 설명</guide><field')
    const composition = {
      snapshot: { version: 1, body: stored, templateId: template.id, legacy: false },
      inputs: { values: { place: '해미연풍우가' }, items: {}, associations: [] },
    }
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [
          {
            ...template,
            compositionBody: converted,
            compositionLegacy: false,
            compositionConverted: true,
          },
        ],
        projects: [
          {
            id: 'owned',
            title: '해미연풍우가',
            videoTemplateId: template.id,
            ratio: 'vertical',
            targetDurationMs: 30000,
            answers: [],
            disclosure: '',
            cta: '',
            composition,
            compositionInputs: composition.inputs,
          },
        ],
      },
    })
    // The conversion is not an edit to offer, and it must not hold the footage the clip needs.
    await waitFor(() => expect(screen.getByLabelText('원본 영상 선택')).toBeEnabled())
    expect(screen.queryByRole('button', { name: '최신 템플릿 적용' })).not.toBeInTheDocument()
  })
  it('opens an older short group at its minimum and saves the stable items only after editing', async () => {
    const writes: ClipProjectDraft[] = []
    const body =
      '<clip version="1" intro="b" caption="bold" outro="e"><group id="menu" label="메뉴" min="2" max="3"><field id="name" label="메뉴 이름" required="true"/></group><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
    const composition = {
      snapshot: { version: 1, body, templateId: template.id, legacy: false },
      inputs: {
        values: {},
        items: { menu: [{ id: 'retained', values: { name: '파스타' } }] },
        associations: [],
      },
    }
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [{ ...template, compositionBody: body }],
        projectWrites: writes,
        projects: [
          {
            id: 'owned',
            title: '내 클립',
            videoTemplateId: template.id,
            ratio: 'vertical',
            targetDurationMs: 30000,
            answers: [],
            disclosure: '',
            cta: '',
            composition,
            compositionInputs: composition.inputs,
          },
        ],
      },
    })
    const names = await screen.findAllByLabelText('메뉴 이름')
    expect(names).toHaveLength(2)
    expect(names[0]).toHaveValue('파스타')
    expect(names[1]).toHaveValue('')
    expect(screen.queryByRole('button', { name: '메뉴 1 삭제' })).not.toBeInTheDocument()
    expect(writes).toHaveLength(0)
    fireEvent.change(names[1], { target: { value: '피자' } })
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionInputs?.items.menu).toEqual([
      { id: 'retained', values: { name: '파스타' } },
      { id: 'minimum_item_1', values: { name: '피자' } },
    ])
    expect(composition.inputs.items.menu).toEqual([{ id: 'retained', values: { name: '파스타' } }])
  })

  it('keeps item facts and stable IDs together when another item is removed; blank prices are optional', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    // The template's inputs are filled in ① beside the sources they describe (CLIP-130).
    const composition = {
      snapshot: {
        version: 1,
        body: CLIP_COMPOSITION_EXAMPLE,
        templateId: template.id,
        legacy: false,
      },
      inputs: { values: {}, items: {}, associations: [] },
    }
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [template],
        projectWrites: writes,
        projects: [
          {
            id: 'owned',
            title: '여러 메뉴',
            videoTemplateId: template.id,
            ratio: 'vertical',
            targetDurationMs: 30000,
            answers: [],
            disclosure: '',
            cta: '',
            composition,
            compositionInputs: composition.inputs,
          },
        ],
      },
    })
    await screen.findByLabelText('클립 제목')
    expect(screen.queryByRole('combobox', { name: /체험단|프리셋/ })).not.toBeInTheDocument()
    // The group holds a required field, so it opens at one item (CLIP-119) and
    // that item cannot be removed; one more makes the pair this case removes from.
    expect(screen.getAllByLabelText('메뉴 이름')).toHaveLength(1)
    expect(screen.queryByRole('button', { name: '메뉴 1 삭제' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '메뉴 추가' }))
    fireEvent.change(screen.getAllByLabelText('메뉴 이름')[0], { target: { value: '파스타' } })
    fireEvent.change(screen.getAllByLabelText('메뉴 이름')[1], { target: { value: '피자' } })
    const secondId = screen.getAllByLabelText('메뉴 이름')[1].id
    await user.click(screen.getByRole('button', { name: '메뉴 1 삭제' }))
    expect(screen.getByLabelText('메뉴 이름')).toHaveValue('피자')
    expect(screen.getByLabelText('메뉴 이름').id).toBe(secondId)
    expect(screen.getByLabelText('가격')).toHaveValue('')
    await waitFor(() => expect(writes.length).toBeGreaterThan(0), { timeout: 4000 })
    expect(writes.at(-1)?.compositionInputs?.items.menu).toEqual([
      { id: expect.any(String), values: { name: '피자' } },
    ])
    expect(writes.at(-1)?.compositionInputs?.values).toEqual({})
    expect(writes.at(-1)?.disclosure).toBe('')
  })
  it('uses field IDs when a label changes and never remaps inputs across template selection', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    const body =
      '<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="바뀐 장소 이름" required="true"/><scene id="scene"/><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
    const first = { ...template, id: 'first', name: '첫 구성', compositionBody: body }
    const second = { ...template, id: 'second', name: '다른 구성', compositionBody: body }
    const composition = {
      snapshot: { version: 1, body, templateId: 'first', legacy: false },
      inputs: { values: { place: '서울' }, items: {}, associations: [] },
    }
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [first, second],
        projectWrites: writes,
        projects: [
          {
            id: 'owned',
            title: '내 클립',
            videoTemplateId: 'first',
            ratio: 'vertical',
            targetDurationMs: 30000,
            answers: [],
            disclosure: '',
            cta: '',
            composition,
            compositionInputs: composition.inputs,
          },
        ],
      },
    })
    expect(await screen.findByLabelText('바뀐 장소 이름')).toHaveValue('서울')
    expect(writes).toHaveLength(0)
    await user.click(screen.getByRole('combobox', { name: /^영상 템플릿/ }))
    await user.click(await screen.findByRole('option', { name: '다른 구성' }))
    expect(screen.getByLabelText('바뀐 장소 이름')).toHaveValue('')
    fireEvent.change(screen.getByLabelText('바뀐 장소 이름'), { target: { value: '제주' } })
    await waitFor(() =>
      expect(writes).toContainEqual(
        expect.objectContaining({
          videoTemplateId: 'second',
          compositionInputs: { values: { place: '제주' }, items: {}, associations: [] },
        }),
      ),
    )
    expect(composition.inputs.values.place).toBe('서울')
  })
  it('applies a changed template explicitly without a save on opening or losing stable inputs', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    const oldBody =
      '<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="장소"/><scene id="scene"/><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>'
    const body = oldBody
      .replace('label="장소"', 'label="촬영 장소"')
      .replace('</clip>', '<field id="extra" label="추가 정보"/></clip>')
    const composition = {
      snapshot: { version: 1, body: oldBody, templateId: template.id, legacy: false },
      inputs: { values: { place: '서울' }, items: {}, associations: [] },
    }
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [{ ...template, compositionBody: body }],
        projectWrites: writes,
        projects: [
          {
            id: 'owned',
            title: '내 클립',
            videoTemplateId: template.id,
            ratio: 'vertical',
            targetDurationMs: 30000,
            answers: [],
            disclosure: '',
            cta: '',
            composition,
            compositionInputs: composition.inputs,
          },
        ],
      },
    })
    expect(await screen.findByLabelText('촬영 장소')).toHaveValue('서울')
    expect(screen.getByLabelText('추가 정보')).toHaveValue('')
    expect(writes).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: '최신 템플릿 적용' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '최신 템플릿 적용' })).not.toBeInTheDocument(),
    )
    expect(writes[0].compositionInputs?.values).toEqual({ place: '서울' })
  })
})
