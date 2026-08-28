import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

describe('VoiceWarning', () => {
  it('renders on an editor only for an empty learned profile', async () => {
    renderAppAt('/posts/new', { user: { id: 'alice' } })
    expect(await screen.findByText(/문체 프로필이 비어 있어요/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '말투 학습하기' })).toHaveAttribute('href', '/voice')
  })

  it('does not render after a styleguide exists', async () => {
    renderAppAt('/posts/new', {
      user: { id: 'alice' },
      voice: { styleguide: '# 종결어미' },
    })
    await screen.findByLabelText('제목')
    expect(screen.queryByText(/문체 프로필이 비어 있어요/)).not.toBeInTheDocument()
  })
})
