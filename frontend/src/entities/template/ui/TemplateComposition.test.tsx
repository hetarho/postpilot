import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TEMPLATE_ASK_MAX_PER_BODY } from '../config'
import { TEMPLATE_LIMITS } from '../model/types'
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

  // TEMPLATE-30: an unreadable body now has a way to FIX as well as a way to discard, and the
  // fix comes first — it keeps what the author wrote.
  it('offers to fix an unreadable body in the source before offering to clear it', async () => {
    const user = userEvent.setup()
    const onFixInSource = vi.fn()
    render(
      <TemplateComposition
        value="<write>닫히지 않음"
        onChange={() => {}}
        onFixInSource={onFixInSource}
      />,
    )

    const actions = screen.getAllByRole('button')
    expect(actions[0]).toHaveAccessibleName('원문에서 고치기')
    expect(actions[1]).toHaveAccessibleName('구성 비우고 다시 만들기')

    await user.click(actions[0])
    expect(onFixInSource).toHaveBeenCalledTimes(1)
  })

  // Without a source mode to switch to, the clear action is the only one — and it must not be
  // rendered as a dead button beside a missing one.
  it('offers only the clear action when there is no source mode', () => {
    render(<TemplateComposition value="<write>닫히지 않음" onChange={() => {}} />)
    expect(screen.queryByRole('button', { name: '원문에서 고치기' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '구성 비우고 다시 만들기' })).toBeInTheDocument()
  })
})

describe('데이터 받기', () => {
  const switchOn = (rowIndex: number) => within(rows()[rowIndex]).getByRole('switch')

  it('is offered on the two rows whose text a post can decide, and on no other kind', async () => {
    render(
      <Editor
        initial={'<write>인트로</write>\n고정 문구\n<slot kind="photo"/>\n<note>메모</note>'}
      />,
    )
    for (const [index, offered] of [
      [0, true],
      [1, true],
      [2, false],
      [3, false],
    ] as const) {
      await userEvent.click(toggle(index))
      const control = within(rows()[index]).queryByRole('switch')
      expect(Boolean(control), `row ${index}`).toBe(offered)
      await userEvent.click(toggle(index))
    }
  })

  it('turns an AI가 쓰는 글 row into a field the post answers, keeping its instruction', async () => {
    render(<Editor initial={'<write>별점과 총평</write>'} />)
    await userEvent.click(toggle(0))
    await userEvent.click(switchOn(0))
    // The title is seeded from the row's own text, so a short line becomes the question.
    expect(screen.getByLabelText('입력란 제목')).toHaveValue('별점과 총평')
    expect(body()).toBe('<ask label="별점과 총평">별점과 총평</ask>')

    await userEvent.clear(screen.getByLabelText('입력란 제목'))
    await userEvent.type(screen.getByLabelText('입력란 제목'), '총평 별점')
    expect(body()).toBe('<ask label="총평 별점">별점과 총평</ask>')
    // The instruction is still editable beside the title.
    expect(screen.getByLabelText('무엇을 쓸지')).toHaveValue('별점과 총평')
  })

  it('replaces a 고정 문구 row text with the title, and restores it when switched off', async () => {
    render(<Editor initial={'방문일'} />)
    await userEvent.click(toggle(0))
    await userEvent.click(switchOn(0))
    expect(screen.getByLabelText('입력란 제목')).toHaveValue('방문일')
    // Its own text field is gone: what the author types takes its place.
    expect(screen.queryByLabelText('글에 들어갈 문구')).not.toBeInTheDocument()
    expect(body()).toBe('<ask label="방문일"/>')

    await userEvent.click(switchOn(0))
    expect(screen.queryByLabelText('입력란 제목')).not.toBeInTheDocument()
    // Nothing ever cleared the row's text, so turning the switch off brings it back.
    expect(body()).toBe('방문일')
  })

  it('shows the switch disabled with its reason inside 사진마다 반복', async () => {
    render(<Editor initial={'<repeat each="photo">\n<write>사진 설명</write>\n</repeat>'} />)
    // Row 0 is the repeat, row 1 its child.
    await userEvent.click(toggle(1))
    const control = switchOn(1)
    expect(control).toBeDisabled()
    expect(within(rows()[1]).getByText(/사진마다 반복 안에서는/)).toBeInTheDocument()
  })

  it('keeps the row kind and marks it, and reads as the question in the outline', async () => {
    render(<Editor initial={'<ask label="총평 별점">별점과 총평</ask>'} />)
    expect(summaries()[0]).toContain('AI가 쓰는 글')
    expect(summaries()[0]).toContain('데이터 받기')
    expect(summaries()[0]).toContain('총평 별점')
  })

  it('says which two rows collided rather than only refusing the body', async () => {
    render(<Editor initial={'<ask label="총평"/>\n<write>인트로</write>'} />)
    await userEvent.click(toggle(1))
    await userEvent.click(switchOn(1))
    await userEvent.clear(screen.getByLabelText('입력란 제목'))
    await userEvent.type(screen.getByLabelText('입력란 제목'), '총평')
    // The parser would refuse the whole body at a line number; the row says it in place.
    expect(within(rows()[1]).getByRole('alert').textContent).toContain('제목')
    expect(screen.getByLabelText('입력란 제목')).toHaveAttribute('aria-invalid', 'true')
  })

  it('round-trips a body that already asks for data', async () => {
    const initial = '오늘의 기록\n<ask label="방문일"/>\n<ask label="총평">총평을 쓰세요</ask>'
    render(<Editor initial={initial} />)
    expect(rows()).toHaveLength(3)
    // Touching a row re-emits the whole body: what the builder writes back is byte-identical to
    // what it read (TEMPLATE-29).
    await userEvent.click(toggle(0))
    expect(body()).toBe(initial)
    expect(summaries()[1]).toContain('방문일')
    expect(summaries()[2]).toContain('총평')
  })
})

// TMPL-50: the title area is its own, smaller composition — words only, on one line.
describe('the title area', () => {
  function TitleEditor({ initial = '' }: { initial?: string }) {
    const [title, setTitle] = useState(initial)
    return (
      <>
        <TemplateComposition area="title_area" value={title} onChange={setTitle} />
        <output data-testid="body">{title}</output>
      </>
    )
  }
  const titlePalette = () => within(screen.getByRole('group', { name: '제목에 추가' }))

  it('offers exactly AI가 쓰는 글 and 고정 문구, under its own name', () => {
    render(<TitleEditor />)
    expect(
      titlePalette()
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual([expect.stringMatching(/^AI가 쓰는 글/), expect.stringMatching(/^고정 문구/)])
    // Not the body's toolbar, which page tests find by its own name.
    expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
    expect(screen.getByText('위에서 블록을 더해 제목을 짜 주세요.')).toBeInTheDocument()
  })

  it('writes its rows on one line, joined by spaces, under its own counter', async () => {
    const user = userEvent.setup()
    render(<TitleEditor />)
    expect(screen.getByText(`${TEMPLATE_LIMITS.titleArea}자 남음`)).toBeInTheDocument()

    await user.click(titlePalette().getByRole('button', { name: /^고정 문구/ }))
    const text = screen.getByLabelText('들어갈 문구')
    // Single-line: a title has no line breaks to type.
    expect(text.tagName).toBe('INPUT')
    await user.type(text, '방문 후기')
    await user.click(titlePalette().getByRole('button', { name: /^AI가 쓰는 글/ }))
    await user.type(screen.getByLabelText('무엇을 쓸지'), '메뉴를 한 줄로')

    expect(body()).toBe('방문 후기 <write>메뉴를 한 줄로</write>')
    expect(
      screen.getByText(`${TEMPLATE_LIMITS.titleArea - body().length}자 남음`),
    ).toBeInTheDocument()
  })

  it('lets both rows ask for data', async () => {
    const user = userEvent.setup()
    render(<TitleEditor initial={'<write>메뉴</write> 가게'} />)
    for (const index of [0, 1]) {
      await user.click(toggle(index))
      expect(within(rows()[index]).getByRole('switch')).toBeEnabled()
      await user.click(toggle(index))
    }
  })

  it('names the title when it cannot be read, and clears only it', async () => {
    const user = userEvent.setup()
    render(<TitleEditor initial={'<note>톤</note>'} />)
    expect(screen.getByRole('alert')).toHaveTextContent('제목 형식을 읽을 수 없어요.')
    await user.click(screen.getByRole('button', { name: '제목 비우고 다시 만들기' }))
    expect(body()).toBe('')
  })
})

// TMPL-55: the title reads first in the one namespace, so the BODY row asking under a title the
// title area already uses is the one that yields — and it comes back when the title lets go.
describe('titles the other area asks under', () => {
  function BodyEditor({ initial }: { initial: string }) {
    const [value, setValue] = useState(initial)
    const [taken, setTaken] = useState<ReadonlySet<string>>(new Set())
    const [conflict, setConflict] = useState(false)
    const [shown, setShown] = useState(true)
    return (
      <>
        {shown && (
          <TemplateComposition
            value={value}
            onChange={setValue}
            takenAskTitles={taken}
            onAskConflict={setConflict}
          />
        )}
        <button type="button" onClick={() => setTaken(new Set(['가게 이름']))}>
          take
        </button>
        <button type="button" onClick={() => setShown(false)}>
          hide
        </button>
        <button type="button" onClick={() => setTaken(new Set())}>
          release
        </button>
        <output data-testid="body">{value}</output>
        <output data-testid="conflict">{String(conflict)}</output>
      </>
    )
  }
  const INITIAL = '<ask label="가게 이름"/>\n<write>인트로</write>'

  // A flag it raised must not outlive the composition: gone, it has no rows left to conflict.
  it('reports no conflict once it unmounts', async () => {
    const user = userEvent.setup()
    render(<BodyEditor initial={INITIAL} />)
    await user.click(screen.getByRole('button', { name: 'take' }))
    expect(screen.getByTestId('conflict')).toHaveTextContent('true')
    await user.click(screen.getByRole('button', { name: 'hide' }))
    expect(screen.getByTestId('conflict')).toHaveTextContent('false')
  })

  it('raises the row message and leaves the row out, then puts it back when released', async () => {
    const user = userEvent.setup()
    render(<BodyEditor initial={INITIAL} />)
    expect(body()).toBe(INITIAL)

    await user.click(screen.getByRole('button', { name: 'take' }))
    expect(body()).toBe('<write>인트로</write>')
    expect(screen.getByTestId('conflict')).toHaveTextContent('true')
    await user.click(toggle(0))
    expect(within(rows()[0]).getByRole('alert').textContent).toContain('제목')

    await user.click(screen.getByRole('button', { name: 'release' }))
    expect(body()).toBe(INITIAL)
    expect(screen.getByTestId('conflict')).toHaveTextContent('false')
  })

  it('does not rewrite a stored body when it opens', () => {
    // Hand-written spacing the builder would not produce: opening must leave it exactly as stored.
    const stored = '<write>인트로</write>\n\n\n<write>본문</write>'
    function Opened() {
      const [value, setValue] = useState(stored)
      return (
        <>
          <TemplateComposition
            value={value}
            onChange={setValue}
            takenAskTitles={new Set(['가게 이름'])}
          />
          <output data-testid="body">{value}</output>
        </>
      )
    }
    render(<Opened />)
    expect(body()).toBe(stored)
  })
})

// A failure only the two areas together have — the ceiling counting the title and the body — is
// the screen's to find, and it renders under the area it names.
describe('a failure found across both areas', () => {
  it('renders its reason under the list', () => {
    render(
      <TemplateComposition
        value={'<ask label="하나"/>'}
        onChange={vi.fn()}
        failure={{ line: 1, reason: 'too_many_asks', area: 'body' }}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      `데이터 받기는 최대 ${TEMPLATE_ASK_MAX_PER_BODY}개까지예요`,
    )
  })

  it('leaves a duplicate to the row that states it', () => {
    render(
      <TemplateComposition
        value={'<ask label="하나"/>'}
        onChange={vi.fn()}
        failure={{ line: 1, reason: 'duplicate_ask_label', area: 'body' }}
      />,
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
