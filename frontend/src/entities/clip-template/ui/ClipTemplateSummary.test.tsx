import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ClipTemplateSummary } from './ClipTemplateSummary'
import { emptyClipRecipe } from '../model/types'

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

  it.each(['', '<clip><broken>'])(
    'keeps a usable fallback for an unreadable body',
    (compositionBody) => {
      render(<ClipTemplateSummary template={{ ...template, compositionBody }} />)
      expect(screen.getByText('구성 예시')).toBeInTheDocument()
      expect(screen.getByText('표시할 구성 단계가 없어요.')).toBeInTheDocument()
    },
  )
})
