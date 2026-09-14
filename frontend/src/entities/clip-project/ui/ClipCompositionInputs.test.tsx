import { useState } from 'react'
import i18next from 'i18next'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { parseClipComposition } from '@/entities/clip-template/@x/clip-project'
import type { ClipCompositionInputs } from '../model/composition'
import { emptyCompositionInputs } from '../model/composition-inputs'
import { ClipCompositionInputFields } from './ClipCompositionInputs'

const source = (attributes: string) =>
  `<clip version="1"><group id="menu" ${attributes}><field id="name" label="Name"/></group></clip>`

function Editable({
  attributes,
  initial = emptyCompositionInputs(),
}: {
  attributes: string
  initial?: ClipCompositionInputs
}) {
  const [value, setValue] = useState(initial)
  return (
    <ClipCompositionInputFields
      document={parseClipComposition(source(attributes))}
      value={value}
      onChange={setValue}
    />
  )
}

afterEach(async () => {
  await i18next.changeLanguage('ko')
})

describe('declared item group controls', () => {
  it.each([
    ['ko', '메뉴 추가', '메뉴 1 삭제'],
    ['en', 'Add 메뉴', 'Remove 메뉴 1'],
  ])(
    'names the group and its controls in %s and respects both bounds',
    async (locale, add, remove) => {
      await i18next.changeLanguage(locale)
      render(<Editable attributes='label="메뉴" min="1" max="2"' />)
      const group = screen.getByRole('region', { name: '메뉴' })
      expect(within(group).getByRole('heading', { name: '메뉴' })).toBeInTheDocument()
      expect(screen.getAllByLabelText('Name')).toHaveLength(1)
      expect(screen.queryByRole('button', { name: remove })).not.toBeInTheDocument()
      fireEvent.change(screen.getByLabelText('Name'), { target: { value: '파스타' } })
      const firstId = screen.getByLabelText('Name').id
      fireEvent.click(screen.getByRole('button', { name: add }))
      expect(screen.getAllByLabelText('Name')).toHaveLength(2)
      expect(screen.getAllByLabelText('Name')[0].id).toBe(firstId)
      expect(screen.getByRole('button', { name: add })).toBeDisabled()
      fireEvent.change(screen.getAllByLabelText('Name')[1], { target: { value: '피자' } })
      const retainedId = screen.getAllByLabelText('Name')[1].id
      fireEvent.click(screen.getByRole('button', { name: remove }))
      expect(screen.getByLabelText('Name')).toHaveValue('피자')
      expect(screen.getByLabelText('Name').id).toBe(retainedId)
      expect(screen.queryByRole('button', { name: remove })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: add })).toBeEnabled()
    },
  )

  it('opens an older short draft without writes, collisions or mutation and persists displayed IDs on edit', () => {
    const initial: ClipCompositionInputs = {
      values: {},
      items: { menu: [{ id: 'minimum_item_1', values: { name: '기존 메뉴' } }] },
      associations: [
        {
          groupId: 'menu',
          itemId: 'minimum_item_1',
          sourceId: 'source',
          fingerprint: 'sha',
          startMs: 0,
          endMs: 1000,
        },
      ],
    }
    const before = structuredClone(initial),
      onChange = vi.fn()
    const document = parseClipComposition(source('label="메뉴" min="3" max="4"'))
    const { rerender } = render(
      <ClipCompositionInputFields document={document} value={initial} onChange={onChange} />,
    )
    const ids = screen.getAllByLabelText('Name').map((input) => input.id)
    expect(ids).toHaveLength(3)
    expect(new Set(ids).size).toBe(3)
    rerender(<ClipCompositionInputFields document={document} value={initial} onChange={onChange} />)
    expect(screen.getAllByLabelText('Name').map((input) => input.id)).toEqual(ids)
    expect(onChange).not.toHaveBeenCalled()
    expect(initial).toEqual(before)
    fireEvent.change(screen.getAllByLabelText('Name')[1], { target: { value: '새 메뉴' } })
    const changed = onChange.mock.calls[0][0] as ClipCompositionInputs
    expect(changed.items.menu).toEqual([
      before.items.menu[0],
      { id: 'minimum_item_2', values: { name: '새 메뉴' } },
      { id: 'minimum_item_3', values: {} },
    ])
    expect(changed.associations).toEqual(before.associations)
    expect(initial).toEqual(before)
  })

  it('keeps the undeclared generic group and controls with no initial items', () => {
    render(<Editable attributes="" />)
    expect(screen.getByRole('heading', { name: '항목 묶음 1' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '항목 추가' }))
    expect(screen.getByLabelText('Name')).toHaveValue('')
    fireEvent.click(screen.getByRole('button', { name: '항목 1 삭제' }))
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument()
  })

  it('does not add to a group whose declared maximum is zero', () => {
    render(<Editable attributes='label="메뉴" max="0"' />)
    expect(screen.getByRole('button', { name: '메뉴 추가' })).toBeDisabled()
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument()
  })
})
