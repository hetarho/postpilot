import { expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

// A video template's card reads like a post template's: its glyph, its name as the link, and one
// line naming the stages it runs through — not a preview of the composition.
it('describes a stored video template by its stages while keeping navigation and deletion separate', async () => {
  renderAppAt('/video-templates', {
    user: { id: 'alice' },
    clips: {
      templates: [
        {
          id: 'visual',
          name: '나만의 영상 구성',
          compositionBody:
            '<clip version="1"><stage name="첫인상">첫 장면</stage><stage name="대표 메뉴">메뉴</stage></clip>',
          introPreset: 'cover',
          outroPreset: 'e',
          allowedCaptionStyles: ['neon'],
          projectCount: 3,
        },
      ],
    },
  })
  const directory = within(await screen.findByRole('region', { name: '저장된 영상 템플릿' }))
  const link = directory.getByRole('link', { name: '나만의 영상 구성' })
  expect(link).toHaveAttribute('href', '/video-templates/visual')
  expect(link).toHaveTextContent(/^나만의 영상 구성$/)
  const card = link.closest('li')!
  expect(card.querySelector('svg.lucide-clapperboard')).not.toBeNull()
  expect(within(card).getByText('첫인상 · 대표 메뉴')).toBeInTheDocument()
  expect(within(card).queryByText('구성 예시')).toBeNull()
  expect(card).not.toHaveTextContent('인트로 매거진 커버')
  expect(directory.getByText('영상 3개에 사용')).toBeInTheDocument()
  expect(directory.getByRole('button', { name: '나만의 영상 구성 삭제' }).closest('a')).toBeNull()
  expect(within(link).queryByRole('button')).not.toBeInTheDocument()
})
