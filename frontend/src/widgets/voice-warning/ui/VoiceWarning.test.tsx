import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

describe('VoiceWarning', () => {
  it('renders on an editor only for an empty learned profile, and links to that voice', async () => {
    renderAppAt('/posts/new', { user: { id: 'alice' } })
    expect(await screen.findByText(/문체 프로필이 비어 있어요/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '말투 학습하기' })).toHaveAttribute(
      'href',
      '/voices/voice-default',
    )
  })

  it('does not render once a profile version exists', async () => {
    renderAppAt('/posts/new', {
      user: { id: 'alice' },
      voice: { structured: { meta: { version: 1n }, empty: false } },
    })
    await screen.findByLabelText('제목')
    expect(screen.queryByText(/문체 프로필이 비어 있어요/)).not.toBeInTheDocument()
  })
})
