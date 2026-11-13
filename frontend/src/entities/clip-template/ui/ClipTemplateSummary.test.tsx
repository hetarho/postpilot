import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ClipTemplateSummary } from './ClipTemplateSummary'
import { emptyClipRecipe } from '../model/types'
import { ChoiceButton } from '@/shared/ui'

const template = {
  ...emptyClipRecipe(),
  name: '여행',
  introPreset: 'a' as const,
  outroPreset: 'b' as const,
  allowedCaptionStyles: ['bold'],
}

describe('ClipTemplateSummary', () => {
  it('shows stored stages and design in a labelled static example', () => {
    render(
      <ClipTemplateSummary
        template={{
          ...template,
          compositionBody:
            '<clip version="1"><stage name="첫인상">도착</stage><stage name="풍경">둘러보기</stage></clip>',
        }}
      />,
    )
    expect(screen.getByText('구성 예시')).toBeInTheDocument()
    expect(screen.getByText('첫인상 → 풍경')).toBeInTheDocument()
    expect(
      screen.getByText(/인트로 심플 제목 · 자막 크게 강조 · 아웃트로 가로선 구분/),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows text-only outlines and distinguishes their stored content', () => {
    const view = render(
      <ClipTemplateSummary
        template={{
          ...template,
          compositionBody:
            '<clip version="1"><text id="caption" kind="fixed" role="caption" basis="whole">카페 소개</text></clip>',
        }}
      />,
    )
    expect(screen.getAllByText('자막: 카페 소개')).toHaveLength(2)
    expect(screen.queryByText('표시할 구성이 없어요.')).not.toBeInTheDocument()
    view.rerender(
      <ClipTemplateSummary
        template={{
          ...template,
          compositionBody:
            '<clip version="1"><text id="caption" kind="fixed" role="caption" basis="whole">여행 소개</text></clip>',
        }}
      />,
    )
    expect(screen.getAllByText('자막: 여행 소개')).toHaveLength(2)
  })

  it('summarizes mixed stages, bound text and disclosure in stored order', () => {
    render(
      <ClipTemplateSummary
        template={{
          ...template,
          compositionBody:
            '<clip version="1"><field id="place" label="장소"/><stage name="도착">첫인상</stage><text id="caption" kind="fixed" role="caption" basis="whole"><value field="place"/></text><text id="badge" kind="fixed" role="badge" basis="whole">광고</text><stage name="마무리">끝</stage></clip>',
        }}
      />,
    )
    expect(screen.getByText('도착 → 자막: 장소 → 공개 문구: 광고 → 마무리')).toBeInTheDocument()
  })

  it('keeps all example regions present with a long valid caption', () => {
    const caption = '긴 문구'.repeat(35)
    render(
      <ClipTemplateSummary
        template={{
          ...template,
          compositionBody: `<clip version="1"><text id="caption" kind="fixed" role="caption" basis="whole">${caption}</text></clip>`,
        }}
      />,
    )
    const example = screen.getByText('구성 예시').parentElement!
    expect(within(example).getByText(/인트로.*심플 제목/)).toBeVisible()
    expect(within(example).getByText(/^자막: 긴 문구.*…$/)).toBeVisible()
    expect(within(example).getByText(/아웃트로.*가로선 구분/)).toBeVisible()
    expect(screen.queryByText(`자막: ${caption}`)).not.toBeInTheDocument()
  })

  it('keeps multiple long choices distinct and their spoken outlines short and ordered', () => {
    const body = (topic: string) =>
      `<clip version="1"><stage name="시작">도입</stage>${[1, 2, 3, 4, 5].map((n) => `<text id="text${n}" kind="fixed" role="caption" basis="whole">${topic}${n}${'긴 내용'.repeat(30)}</text>`).join('')}</clip>`
    render(
      <>
        <ChoiceButton
          title="카페"
          selected={false}
          description={
            <ClipTemplateSummary template={{ ...template, compositionBody: body('카페') }} />
          }
        />
        <ChoiceButton
          title="여행"
          selected={false}
          description={
            <ClipTemplateSummary template={{ ...template, compositionBody: body('여행') }} />
          }
        />
      </>,
    )
    for (const name of ['카페', '여행']) {
      const choice = screen.getByRole('button', { name })
      const description = choice.getAttribute('aria-describedby')!
      const spoken = document.getElementById(description)!.textContent!
      expect(spoken).toContain(`시작 → 자막: ${name}1`)
      expect(spoken).toContain(`자막: ${name}2`)
      expect(spoken).not.toContain(`${name}5`)
      expect(spoken.length).toBeLessThan(300)
    }
  })

  it.each(['', '<clip><broken>'])(
    'keeps a usable fallback for an unreadable body',
    (compositionBody) => {
      render(<ClipTemplateSummary template={{ ...template, compositionBody }} />)
      expect(screen.getByText('구성 예시')).toBeInTheDocument()
      expect(screen.getByText('표시할 구성이 없어요.')).toBeInTheDocument()
    },
  )
})
