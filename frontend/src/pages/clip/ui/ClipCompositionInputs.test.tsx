import { afterEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { CLIP_COMPOSITION_EXAMPLE, type ClipRecipe } from '@/entities/clip-template'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const template = {
  id: 'menu',
  name: '여러 메뉴',
  compositionBody: CLIP_COMPOSITION_EXAMPLE,
  compositionLegacy: false,
  informationFields: [],
  cutGuidance: '',
  copyStyles: ['clean'] as ClipRecipe['copyStyles'],
  accent: '' as const,
  preset: '' as const,
}
afterEach(() => discardClipDraftQueues())

describe('template-defined project inputs', () => {
  it('keeps item facts and stable IDs together when another item is removed; blank prices are optional', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    renderAppAt('/clips/new', {
      user: { id: 'alice' },
      clips: { templates: [template], projectWrites: writes },
    })
    await user.type(await screen.findByLabelText('클립 제목'), '여러 메뉴')
    await user.click(screen.getByRole('combobox', { name: /^영상 템플릿/ }))
    await user.click(await screen.findByRole('option', { name: '여러 메뉴' }))
    expect(screen.queryByRole('combobox', { name: /체험단|프리셋/ })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '항목 추가' }))
    await user.click(screen.getByRole('button', { name: '항목 추가' }))
    fireEvent.change(screen.getAllByLabelText('메뉴 이름')[0], { target: { value: '파스타' } })
    fireEvent.change(screen.getAllByLabelText('메뉴 이름')[1], { target: { value: '피자' } })
    const secondId = screen.getAllByLabelText('메뉴 이름')[1].id
    await user.click(screen.getByRole('button', { name: '항목 1 삭제' }))
    expect(screen.getByLabelText('메뉴 이름')).toHaveValue('피자')
    expect(screen.getByLabelText('메뉴 이름').id).toBe(secondId)
    expect(screen.getByLabelText('가격')).toHaveValue('')
    await user.click(screen.getByRole('button', { name: '클립 만들기' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionInputs?.items.menu).toEqual([
      { id: expect.any(String), values: { name: '피자' } },
    ])
    expect(writes[0].compositionInputs?.values).toEqual({})
    expect(writes[0].disclosure).toBe('')
  })
  it('uses field IDs when a label changes and never remaps inputs across template selection', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    const body =
      '<clip version="1"><field id="place" label="바뀐 장소 이름" required="true"/><scene id="scene"/></clip>'
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
    const oldBody = '<clip version="1"><field id="place" label="장소"/><scene id="scene"/></clip>'
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
