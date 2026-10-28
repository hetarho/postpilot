import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import type { SectionDraft } from '../model/wizard-machine'
import { PreviewDock } from './PreviewDock'
import { WizardPreview } from './WizardPreview'

const section = (patch: Partial<SectionDraft>): SectionDraft => ({
  id: 'visit',
  name: '방문 계기',
  help: '찾아간 이유',
  checked: true,
  heading: '방문 계기',
  content: 'write',
  question: '방문 계기',
  required: false,
  literal: '',
  photos: 0,
  repeatable: false,
  repeat: false,
  ...patch,
})

const preview = () => within(screen.getByRole('article', { name: '미리보기' }))

// TMPL-75 / TMPL-66: the wizard draws the post its answers make, headings as 소제목.
describe('the wizard preview', () => {
  it('draws the checked sections in order, each heading as a 소제목', () => {
    render(
      <WizardPreview
        sections={[
          section({ id: 'visit', heading: '📝 어쩌다 가게 됐냐면요' }),
          section({ id: 'hidden', heading: '빠진 부분', checked: false }),
          section({ id: 'menu', name: '주문한 메뉴', help: '먹은 메뉴', heading: '' }),
        ]}
      />,
    )
    expect(
      preview()
        .getAllByRole('heading', { level: 4 })
        .map((heading) => heading.textContent),
    ).toEqual(['📝 어쩌다 가게 됐냐면요', '주문한 메뉴'])
    expect(preview().getByText('제목은 AI가 지어요')).toBeInTheDocument()
    expect(preview().getByText('찾아간 이유')).toBeInTheDocument()
  })

  it('marks the section whose step is on screen', () => {
    render(
      <WizardPreview
        sections={[section({ id: 'visit' }), section({ id: 'menu', heading: '메뉴' })]}
        currentId="menu"
      />,
    )
    const current = preview()
      .getAllByRole('heading', { level: 4 })
      .find((heading) => heading.closest('[aria-current="step"]'))
    expect(current).toHaveTextContent('메뉴')
  })

  it('draws photos as a small strip and a repeating section twice', () => {
    render(
      <WizardPreview
        sections={[
          section({ id: 'menu', heading: '메뉴', photos: 2, repeatable: true, repeat: true }),
        ]}
      />,
    )
    expect(preview().getAllByRole('img', { name: '사진 2장' })).toHaveLength(2)
    expect(preview().getAllByText('사진 그룹마다 반복')).toHaveLength(1)
    expect(preview().getAllByText('찾아간 이유')).toHaveLength(2)
  })

  it('shows a typed answer as its placeholder and fixed text as written', () => {
    render(
      <WizardPreview
        sections={[
          section({
            id: 'info',
            heading: '매장 정보',
            content: 'ask',
            question: '주소와 영업시간',
          }),
          section({ id: 'map', heading: '지도', content: 'literal', literal: '지도는 아래에' }),
          section({ id: 'end', heading: '끝인사', content: 'literal', literal: ' ' }),
        ]}
      />,
    )
    expect(preview().getByText('주소와 영업시간')).toBeInTheDocument()
    expect(preview().getByText('입력한 내용')).toBeInTheDocument()
    expect(preview().getByText('지도는 아래에')).toBeInTheDocument()
    expect(preview().getByText('적어 둔 문구가 그대로 들어가요')).toBeInTheDocument()
  })
})

describe('the phone preview button', () => {
  it('opens the preview in a sheet and glows only after a change', () => {
    const { rerender } = render(
      <PreviewDock changes={0}>
        <p>미리 본 글</p>
      </PreviewDock>,
    )
    expect(screen.queryByTestId('preview-glow')).not.toBeInTheDocument()
    expect(screen.queryByText('미리 본 글')).not.toBeInTheDocument()
    rerender(
      <PreviewDock changes={1}>
        <p>미리 본 글</p>
      </PreviewDock>,
    )
    expect(screen.getByTestId('preview-glow')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '미리보기' }))
    expect(screen.getByRole('dialog', { name: '미리보기' })).toHaveTextContent('미리 본 글')
  })
})
