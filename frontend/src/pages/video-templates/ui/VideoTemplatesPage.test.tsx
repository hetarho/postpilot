import { expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

it('shows stored video composition and design while keeping navigation and deletion separate', async () => {
  renderAppAt('/video-templates?channel=instagram_clip', {
    user: { id: 'alice' },
    clips: {
      templates: [
        {
          id: 'visual',
          name: '나만의 영상 구성',
          compositionBody: '<clip version="1"><stage name="첫인상">첫 장면</stage></clip>',
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
  expect(link).toHaveAttribute('href', '/video-templates/visual?channel=instagram_clip')
  expect(within(link).getByText('구성 예시')).toBeInTheDocument()
  expect(link).toHaveTextContent('첫인상')
  expect(link).toHaveTextContent('인트로 매거진 커버 · 자막 네온 사인 · 아웃트로 점수 강조')
  expect(directory.getByText('클립 3개')).toBeInTheDocument()
  expect(directory.getByRole('button', { name: '나만의 영상 구성 삭제' }).closest('a')).toBeNull()
  expect(within(link).queryByRole('button')).not.toBeInTheDocument()
})
