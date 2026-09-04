import { beforeEach, describe, expect, it } from 'vitest'
import { useState } from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { TemplateBodyEditor } from './TemplateBodyEditor'

// The editor is pure UI over one string: it needs the copy and nothing else — no transport, no
// query client, no session.
beforeEach(() => {
  initializeI18n('ko')
})

function Harness({ initial = '' }: { initial?: string }) {
  const [body, setBody] = useState(initial)
  return (
    <>
      <TemplateBodyEditor value={body} onChange={setBody} />
      <output data-testid="body">{body}</output>
    </>
  )
}

const body = () => screen.getByTestId('body').textContent

describe('the template body editor', () => {
  it('adds a block from the palette and writes it into the body', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    await user.click(
      within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
        name: '작성',
      }),
    )
    await user.type(screen.getByLabelText('무엇을 쓸지'), '인트로')

    expect(body()).toBe('<write>인트로</write>')
  })

  it('keeps the source and the structure views over one string', async () => {
    const user = userEvent.setup()
    render(<Harness initial={'<write>인트로</write>\n=====\n<slot kind="place" label="지도"/>'} />)

    // The structure view reads the same body: three rows, in order.
    expect(screen.getByLabelText('무엇을 쓸지')).toHaveValue('인트로')
    expect(screen.getByLabelText('들어갈 문구')).toHaveValue('=====')
    expect(screen.getByLabelText('이 자리의 이름')).toHaveValue('지도')

    await user.click(screen.getByRole('tab', { name: '원문' }))
    expect(screen.getByLabelText('템플릿 구성')).toHaveValue(
      '<write>인트로</write>\n=====\n<slot kind="place" label="지도"/>',
    )
  })

  it('reorders a block with the move controls the phone can actually use', async () => {
    const user = userEvent.setup()
    render(<Harness initial={'<write>첫째</write>\n<write>둘째</write>'} />)

    const rows = screen.getAllByRole('listitem')
    await user.click(within(rows[1]).getByRole('button', { name: '위로' }))

    expect(body()).toBe('<write>둘째</write>\n<write>첫째</write>')
  })

  it('forces the source view and names the line when the body does not parse', async () => {
    render(<Harness initial={'<repaet each="photo">\n</repaet>'} />)

    expect(screen.getByRole('alert')).toHaveTextContent('1번째 줄을 읽을 수 없어요')
    // The structure view is not offered for bytes it could not read.
    expect(screen.getByRole('tab', { name: '구성 편집' })).toBeDisabled()
    expect(screen.getByLabelText('템플릿 구성')).toBeInTheDocument()
  })

  it('does not offer a repeat inside a repeat, because the grammar forbids one', () => {
    render(<Harness initial={'<repeat each="photo">\n<write>설명</write>\n</repeat>'} />)

    const inner = screen.getByRole('group', { name: '안쪽에 추가' })
    const options = within(inner)
      .getAllByRole('button')
      .map((option) => option.textContent)
    expect(options).not.toContain('반복')
    // The outer palette still offers one — it is only nesting that the grammar forbids.
    const outer = screen.getByRole('group', { name: '블록 추가' })
    expect(
      within(outer)
        .getAllByRole('button')
        .map((option) => option.textContent),
    ).toContain('반복')
  })
})
