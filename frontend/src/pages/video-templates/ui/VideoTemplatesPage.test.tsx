import { expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

// A video template's card is the post template's card: its name as the one link, then the video it
// makes — the opening and closing design around one example line, and its stages in order — and
// how many clips use it. Deleting lives on the template's own screen, as for a post template.
it('draws a stored video template as the video it makes, one link and nothing else to press', async () => {
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
  expect(within(link).getByText('구성 예시')).toBeInTheDocument()
  expect(within(link).getByText(/첫인상 → 대표 메뉴/)).toBeInTheDocument()
  expect(link).toHaveTextContent('인트로 매거진 커버')
  expect(directory.getByText('클립 3개')).toBeInTheDocument()
  expect(directory.queryByRole('button')).toBeNull()
})
