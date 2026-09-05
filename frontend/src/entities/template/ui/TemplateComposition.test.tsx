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

const REVIEW =
  '<write>인트로를 씁니다</write>\n<slot kind="place" label="네이버 지도"/>\n' +
  '<repeat each="photo">\n<slot kind="photo"/>\n<write>이 사진에 대한 설명</write>\n</repeat>\n' +
  '<write>총평 및 재방문 의사</write>'

describe('the composition editor', () => {
  // A5: the rows ARE the outline — one line per block, a repeat's children beneath it, and the
  // whole shape readable without expanding anything.
  it('reads as the outline of the post with nothing expanded', () => {
    render(<Editor initial={REVIEW} />)

    expect(summaries()).toEqual([
      '작성인트로를 씁니다',
      '지도·장소네이버 지도',
      '반복사진마다 되풀이',
      '사진첨부한 사진이 들어갑니다',
      '작성이 사진에 대한 설명',
      '작성총평 및 재방문 의사',
    ])
    // A9: nothing of the grammar reaches the screen.
    for (const syntax of ['<write', '<repeat', '<slot', '<note', 'each="photo"']) {
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
    expect(screen.getByLabelText('이 자리의 이름')).toHaveValue('네이버 지도')

    await user.clear(screen.getByLabelText('이 자리의 이름'))
    await user.type(screen.getByLabelText('이 자리의 이름'), '카카오맵')
    expect(body()).toContain('<slot kind="place" label="카카오맵"/>')
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

    await user.click(screen.getByRole('button', { name: '메모' }))
    expect(summaries()[1]).toContain('메모')
  })

  // A7 second half: the aim inside a repeat puts the block inside it.
  it('adds inside a repeat when the aim is a row inside one', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    // The repeat's first child.
    await user.click(toggle(3))
    await user.click(screen.getByRole('button', { name: '정해진 문구' }))
    await user.type(screen.getByLabelText('들어갈 문구'), '사진 아래 한 줄')

    expect(body()).toContain(
      '<repeat each="photo">\n<slot kind="photo"/>\n사진 아래 한 줄\n<write>이 사진에 대한 설명</write>\n</repeat>',
    )
  })

  // A8: the grammar forbids a repeat inside a repeat, so the command is not even offered there.
  it('offers no repeat while the aim is inside a repeat', async () => {
    const user = userEvent.setup()
    render(<Editor initial={REVIEW} />)

    expect(screen.getByRole('button', { name: '반복' })).toBeInTheDocument()
    await user.click(toggle(4))
    expect(screen.queryByRole('button', { name: '반복' })).not.toBeInTheDocument()
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

    expect(summaries()[0]).toContain('네이버 지도')
    expect(summaries()[1]).toContain('인트로를 씁니다')
    // The body follows, so a drag is a real edit and not just a rearranged view.
    expect(body().startsWith('<slot kind="place" label="네이버 지도"/>')).toBe(true)
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
    expect(summaries()[0]).toContain('네이버 지도')

    // The repeat's last child cannot move down past the repeat: it is the last of ITS group.
    expect(within(rows()[4]).getByRole('button', { name: '아래로' })).toBeDisabled()
  })

  // A block exists as a row the moment it is added, before anything is typed into it — but it
  // contributes no bytes until it says something, because an empty `<write></write>` does not
  // parse and the editor must never emit a body its own parser refuses.
  it('keeps an empty new block as a row while contributing nothing to the body', async () => {
    const user = userEvent.setup()
    render(<Editor />)

    await user.click(screen.getByRole('button', { name: '작성' }))
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
    expect(summaries()[0]).toContain('네이버 지도')
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

    await user.click(screen.getByRole('button', { name: '메모' }))
    expect(summaries()[summaries().length - 1]).toContain('메모')
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
