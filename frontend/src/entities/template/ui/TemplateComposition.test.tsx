import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TemplateComposition } from './TemplateComposition'

/** The editor is a controlled input, so every test drives it through a real parent — which is
 *  also what proves the body it emits is what a caller would save. */
function Editor({ initial = '' }: { initial?: string }) {
  const [body, setBody] = useState(initial)
  return (
    <>
      <TemplateComposition value={body} onChange={setBody} />
      {/* The emitted body, read back by the assertions. It is never shown to a user. */}
      <output data-testid="body">{body}</output>
    </>
  )
}

const body = () => screen.getByTestId('body').textContent ?? ''
/** Every block's row, in reading order — a repeat's children are nested inside its own item, so
 *  the flat DOM order IS the outline the user sees. */
const rows = () => screen.getAllByRole('listitem')
/** A row's own toggle. It is the first button inside the item: the drag handle is a span, and the
 *  move controls come after the content. */
const toggle = (index: number) => within(rows()[index]).getAllByRole('button')[0]
const summaries = () => rows().map((_, index) => toggle(index).textContent ?? '')
/** A palette button by its NAME, scoped to the toolbar: a row's badge carries the SAME name as
 *  the button that creates it (which is the point), so an unscoped query matches both. The name
 *  is matched by its first line because the accessible name now carries the visible help too. */
const palette = () => within(screen.getByRole('group', { name: '블록 추가' }))
const paletteButton = (name: string) =>
  palette().getByRole('button', { name: new RegExp(`^${name}`) })
const queryPaletteButton = (name: string) =>
  palette().queryByRole('button', { name: new RegExp(`^${name}`) })

const REVIEW =
  '<write>인트로를 씁니다</write>\n지도는 아래에\n' +
  '<repeat each="photo">\n<slot kind="photo"/>\n<write>이 사진에 대한 설명</write>\n</repeat>\n' +
  '<write>총평 및 재방문 의사</write>'

/** A body from before the place and link positions were retired (TEMPLATE-37). */
const LEGACY =
  '<write>인트로를 씁니다</write>\n<slot kind="place" label="네이버 지도"/>\n<slot kind="link"/>'

describe('the composition editor', () => {
  // A5: the rows ARE the outline — one line per block, a repeat's children beneath it, and the
  // whole shape readable without expanding anything.
  it('reads as the outline of the post with nothing expanded', () => {
    render(<Editor initial={REVIEW} />)

    expect(summaries()).toEqual([
      'AI가 쓰는 글인트로를 씁니다',
      '고정 문구지도는 아래에',
      '사진마다 반복사진마다 되풀이',
      '사진사진 1장',
      'AI가 쓰는 글이 사진에 대한 설명',
      'AI가 쓰는 글총평 및 재방문 의사',
    ])
    // A9: nothing of the grammar reaches the screen.
    for (const syntax of ['<write', '<repeat', '<slot', '<note', 'each="photo"', 'count="']) {
      expect(
        screen.getByRole('group', { name: '블록 추가' }).closest('div')?.textContent,
      ).not.toContain(syntax)
    }
    // No fields are open, so no editable control is mounted yet.
    expect(screen.queryByLabelText('무엇을 쓸지')).not.toBeInTheDocument()
  })

  // A11: an untouched composition emits the byte-identical body it was given.
  it('round-trips an untouched composition byte for byte', () => {
    render(<Editor initial={REVIEW} />)
    expect(body()).toBe(REVIEW)
  })

  // A6: one row opens at a time, and the previous one closes.
  it('expands one row at a time and edits it in place', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    await user.click(toggle(0))
    expect(screen.getByLabelText('무엇을 쓸지')).toHaveValue('인트로를 씁니다')

    await user.click(toggle(1))
    // The first row's field is gone: at most one is open.
    expect(screen.queryByLabelText('무엇을 쓸지')).not.toBeInTheDocument()
    expect(screen.getByLabelText('들어갈 문구')).toHaveValue('지도는 아래에')

    await user.clear(screen.getByLabelText('들어갈 문구'))
    await user.type(screen.getByLabelText('들어갈 문구'), '지도는 맨 아래')
    expect(body()).toContain('지도는 맨 아래')
  })

  // TEMPLATE-38: a photo position carries how many photos stand side by side, edited with a
  // stepper because the values are single digits inside a hard range.
  it('edits a photo row count with a stepper bounded by the configured ceiling', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    await user.click(toggle(3))
    const value = screen.getByRole('spinbutton', { name: '가로로 놓을 사진 수' })
    expect(value).toHaveAttribute('aria-valuenow', '1')
    expect(value).toHaveAttribute('aria-valuemin', '1')
    expect(value).toHaveAttribute('aria-valuemax', '4')
    // At the floor there is nothing to take away.
    expect(screen.getByRole('button', { name: '줄이기' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: '늘리기' }))
    expect(body()).toContain('<slot kind="photo" count="2"/>')
    // The collapsed summary says they stand side by side, which is the point of the count.
    expect(summaries()[3]).toBe('사진사진 2장 가로로')
    // And the repeat's help states what one iteration now takes.
    await user.click(toggle(2))
    expect(screen.getByText(/한 번 되풀이할 때 사진 2장을 씁니다/)).toBeInTheDocument()

    await user.click(toggle(3))
    await user.click(screen.getByRole('button', { name: '늘리기' }))
    await user.click(screen.getByRole('button', { name: '늘리기' }))
    expect(body()).toContain('<slot kind="photo" count="4"/>')
    // The ceiling is the server's, so the control cannot offer a value the save would refuse.
    expect(screen.getByRole('button', { name: '늘리기' })).toBeDisabled()
  })

  // TEMPLATE-37: the position is retired, but a stored one must not make the body unreadable.
  // It opens as 고정 문구 carrying its label, and the save writes it back as literal text.
  it('opens a stored place or link position as fixed text', async () => {
    const user = userEvent.setup()
    render(<Editor initial={LEGACY} />)

    expect(summaries()).toEqual([
      'AI가 쓰는 글인트로를 씁니다',
      '고정 문구네이버 지도',
      // An unlabelled one falls back to a name rather than opening empty.
      '고정 문구링크',
    ])
    // Nothing is emitted until the user actually edits: the migration is not a save of its own.
    expect(body()).toBe(LEGACY)

    await user.click(toggle(1))
    await user.type(screen.getByLabelText('들어갈 문구'), ' 참고')
    expect(body()).toBe('<write>인트로를 씁니다</write>\n네이버 지도 참고\n링크')
    expect(body()).not.toContain('<slot')
  })

  // A7: the toolbar lands where the screen said, and the aim is drawn BEFORE the click.
  it('adds at the current position and shows where that is', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    // With nothing touched the aim is the end, so the marker sits past the last row.
    expect(screen.getByText('여기에 추가돼요')).toBeInTheDocument()

    // Touching the first row moves the aim to just after it — visibly, before anything is added.
    await user.click(toggle(0))
    const marked = rows().findIndex((row) => within(row).queryByText('여기에 추가돼요') !== null)
    expect(marked).toBe(0)

    await user.click(paletteButton('AI에게만 하는 말'))
    expect(summaries()[1]).toContain('AI에게만 하는 말')
  })

  // A7 second half: the aim inside a repeat puts the block inside it.
  it('adds inside a repeat when the aim is a row inside one', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    // The repeat's first child.
    await user.click(toggle(3))
    await user.click(paletteButton('고정 문구'))
    await user.type(screen.getByLabelText('들어갈 문구'), '사진 아래 한 줄')

    expect(body()).toContain(
      '<repeat each="photo">\n<slot kind="photo"/>\n사진 아래 한 줄\n<write>이 사진에 대한 설명</write>\n</repeat>',
    )
  })

  // A8: the grammar forbids a repeat inside a repeat, so the command is not even offered there.
  it('offers no repeat while the aim is inside a repeat', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    expect(paletteButton('사진마다 반복')).toBeInTheDocument()
    await user.click(toggle(4))
    expect(queryPaletteButton('사진마다 반복')).not.toBeInTheDocument()
  })

  // A8, the other half: pointer drag reorders too. It is not a nicety — the move buttons exist
  // because HTML5 drag events do not fire on touch, so the two are the phone's way and the
  // pointer's way, and neither may be the only one.
  it('reorders by pointer drag', () => {
    render(<Editor initial={REVIEW} />)
    const items = rows()

    fireEvent.dragStart(items[0], { dataTransfer: { setData: () => {}, effectAllowed: '' } })
    fireEvent.dragOver(items[1])
    fireEvent.drop(items[1])

    expect(summaries()[0]).toContain('지도는 아래에')
    expect(summaries()[1]).toContain('인트로를 씁니다')
    // The body follows, so a drag is a real edit and not just a rearranged view.
    expect(body().startsWith('지도는 아래에')).toBe(true)
  })

  // A8: a drag cannot take a block out of its repeat — the grammar has no way to express one
  // that left, so the drop targets are its siblings and nothing else.
  it('keeps a dragged child inside its repeat', () => {
    render(<Editor initial={REVIEW} />)
    const items = rows()

    // rows()[4] is the repeat's second child; rows()[5] is the block AFTER the repeat.
    fireEvent.dragStart(items[4], { dataTransfer: { setData: () => {}, effectAllowed: '' } })
    fireEvent.dragOver(items[5])
    fireEvent.drop(items[5])

    expect(body()).toContain(
      '<repeat each="photo">\n<slot kind="photo"/>\n<write>이 사진에 대한 설명</write>\n</repeat>',
    )
    expect(summaries()[5]).toContain('총평 및 재방문 의사')
  })

  // A8: the move buttons reorder within a sibling group and cannot take a block out of one.
  it('reorders with the move buttons, scoped to siblings', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    await user.click(within(rows()[1]).getByRole('button', { name: '위로' }))
    expect(summaries()[0]).toContain('지도는 아래에')

    // The repeat's last child cannot move down past the repeat: it is the last of ITS group.
    expect(within(rows()[4]).getByRole('button', { name: '아래로' })).toBeDisabled()
  })

  // A block exists as a row the moment it is added, before anything is typed into it — but it
  // contributes no bytes until it says something, because an empty `<write></write>` does not
  // parse and the editor must never emit a body its own parser refuses.
  it('keeps an empty new block as a row while contributing nothing to the body', async () => {
    const user = userEvent.setup()
    render(<Editor />)

    await user.click(paletteButton('AI가 쓰는 글'))
    expect(rows()).toHaveLength(1)
    expect(body()).toBe('')

    await user.type(screen.getByLabelText('무엇을 쓸지'), '첫인상')
    expect(body()).toBe('<write>첫인상</write>')
  })

  it('deletes a block from its expanded panel', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    await user.click(toggle(0))
    await user.click(screen.getByRole('button', { name: '삭제' }))
    expect(summaries()[0]).toContain('지도는 아래에')
    expect(body()).not.toContain('인트로를 씁니다')
  })

  // A7: the aim must never point at a block that is gone. Deleting the aimed row used to leave
  // the marker unrendered while the toolbar silently appended at the end — the one failure a
  // single toolbar cannot afford.
  it('falls back to a VISIBLE end position when the aimed row is deleted', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    await user.click(toggle(0))
    // The marker followed the aim onto row 0.
    expect(within(rows()[0]).getByText('여기에 추가돼요')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '삭제' }))
    // Its block is gone, so the aim is the end again — and the end is drawn.
    expect(screen.getByText('여기에 추가돼요')).toBeInTheDocument()
    expect(rows().some((row) => within(row).queryByText('여기에 추가돼요') !== null)).toBe(false)

    await user.click(paletteButton('AI에게만 하는 말'))
    expect(summaries()[summaries().length - 1]).toContain('AI에게만 하는 말')
  })

  // A10: a body the parser cannot read shows no grammar and offers one action.
  it('refuses to guess at an unreadable body and offers only to start over', async () => {
    const user = userEvent.setup()
    render(<Editor initial={'<write>닫히지 않음'} />)

    expect(screen.getByText(/구성을 읽을 수 없어요/)).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '구성 비우고 다시 만들기' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '구성 비우고 다시 만들기' }))
    expect(body()).toBe('')
    expect(screen.getByRole('group', { name: '블록 추가' })).toBeInTheDocument()
  })
})
