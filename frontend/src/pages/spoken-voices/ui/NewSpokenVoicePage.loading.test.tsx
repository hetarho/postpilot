import { render, screen } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import { NewSpokenVoicePage } from './NewSpokenVoicePage'

const state = vi.hoisted(() => ({ pending: true, voices: [] as { id: string }[] }))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/spoken-voices">{children}</a>,
  useNavigate: () => vi.fn(),
  useSearch: () => ({ copy: 'source' }),
}))
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('@/entities/session', () => ({ useSession: () => ({ user: { id: 'owner' } }) }))
vi.mock('@/entities/spoken-voice', () => ({
  useSpokenLibrary: () => ({ voices: state.voices, voicesQuery: { isPending: state.pending } }),
}))
vi.mock('@/widgets/spoken-voice-creation', () => ({ SpokenVoiceCreation: () => <div>form</div> }))

beforeEach(() => {
  state.pending = true
  state.voices = []
})

it('keeps the return and heading while the copied source loads, then reports a missing source', () => {
  const view = render(<NewSpokenVoicePage />)
  expect(screen.getByRole('link', { name: 'back' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'title' })).toBeInTheDocument()
  expect(screen.getByRole('status')).toHaveTextContent('loading')
  expect(screen.queryByText('copyFailed')).not.toBeInTheDocument()
  state.pending = false
  view.rerender(<NewSpokenVoicePage />)
  expect(screen.getByRole('status')).toHaveTextContent('copyFailed')
  expect(screen.queryByText('loading')).not.toBeInTheDocument()
})
