import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { TemplatePreview } from './TemplatePreview'

const preview = () => within(screen.getByRole('article', { name: '미리보기' }))

// TMPL-66: every construct has a stand-in that keeps what is fixed and what the AI writes apart.
describe('the template preview', () => {
  it('draws each construct as its stand-in', () => {
    render(
      <TemplatePreview
        titleArea=""
        body={
          '<write>메뉴 소개</write>\n안녕하세요 고정 인사\n<slot kind="photo" count="3"/>\n' +
          '<ask label="총평 별점"/>\n<ask label="가격">가격 이야기</ask>\n<slot kind="link"/>'
        }
      />,
    )
    // An AI가 쓰는 글 is a grey box naming its topic, never sample prose.
    expect(preview().getByText('메뉴 소개')).toBeInTheDocument()
    expect(preview().getAllByText('AI가 쓰는 글')).toHaveLength(2)
    // A 고정 문구 is its own text.
    expect(preview().getByText('안녕하세요 고정 인사')).toBeInTheDocument()
    // A 사진 is one row of `count` cells.
    const row = preview().getByRole('img', { name: '사진 3장' })
    expect(row.children).toHaveLength(3)
    // A data field shows its 제목 over an 입력한 내용 placeholder, in its flavor's stand-in.
    expect(preview().getByText('총평 별점')).toBeInTheDocument()
    expect(preview().getByText('가격 이야기')).toBeInTheDocument()
    expect(preview().getAllByText(/입력한 내용/)).toHaveLength(2)
    // A stored link position is the 고정 문구 it reads as (TMPL-37).
    expect(preview().getByText('링크')).toBeInTheDocument()
  })

  it('draws a 사진마다 반복 twice under one mark', () => {
    render(
      <TemplatePreview
        titleArea=""
        body={
          '<repeat each="photo">\n<slot kind="photo" count="2"/>\n<write>장면</write>\n</repeat>'
        }
      />,
    )
    expect(preview().getAllByText('사진 그룹마다 반복')).toHaveLength(1)
    expect(preview().getAllByText('장면')).toHaveLength(2)
    expect(preview().getAllByRole('img', { name: '사진 2장' })).toHaveLength(2)
  })

  it('puts the title area first, on one title line', () => {
    render(
      <TemplatePreview
        titleArea={'<ask label="가게 이름"/> 방문 후기 <write>메뉴를 한 줄로</write>'}
        body="<write>인트로</write>"
      />,
    )
    const title = preview().getByRole('heading', { level: 3 })
    expect(title).toHaveTextContent('가게 이름: 입력한 내용 방문 후기 메뉴를 한 줄로')
    expect(
      title.compareDocumentPosition(preview().getByText('인트로')) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
  })

  it('keeps the last parsable state while the text does not parse, and says so', () => {
    const { rerender } = render(<TemplatePreview titleArea="" body="<write>인트로</write>" />)
    expect(preview().getByText('인트로')).toBeInTheDocument()

    rerender(<TemplatePreview titleArea="" body={'<write>인트로</write>\n<write>닫히지'} />)
    expect(preview().getByText('인트로')).toBeInTheDocument()
    expect(preview().getByRole('status')).toHaveTextContent(
      '본문 2번째 줄을 읽을 수 없어 마지막으로 읽힌 모습을 보여 줘요.',
    )
  })

  it('shows only the notice before any text has parsed', () => {
    render(<TemplatePreview titleArea="" body="<write>닫히지" />)
    expect(preview().getByRole('status')).toHaveTextContent('본문 1번째 줄')
    expect(preview().queryByText('닫히지')).not.toBeInTheDocument()
  })

  it('says what it is for when the draft is empty', () => {
    render(<TemplatePreview titleArea="" body="" />)
    expect(
      preview().getByText('구성에 블록을 추가하면 여기에 미리보기가 보여요.'),
    ).toBeInTheDocument()
  })
})
