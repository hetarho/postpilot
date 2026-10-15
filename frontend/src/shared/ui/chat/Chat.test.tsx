import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Chat, ChatComposer } from './Chat'

afterEach(cleanup)

describe('Chat', () => {
  it('renders a labelled conversation and a controlled growing composer with explicit send', async () => {
    const send = vi.fn()
    function Conversation() {
      const [body, setBody] = useState('')
      return (
        <Chat
          label="Conversation"
          partnerLabel="Partner"
          ownerLabel="Me"
          messages={[
            { id: 'question', sender: 'partner', content: 'How was your day?' },
            { id: 'answer', sender: 'owner', content: 'A quiet walk.' },
          ]}
          composer={
            <ChatComposer
              value={body}
              onChange={setBody}
              onSubmit={(event) => {
                event.preventDefault()
                send(body)
              }}
              label="Reply"
              sendLabel="Send"
              disabled={!body.trim()}
            />
          }
        />
      )
    }
    render(<Conversation />)
    expect(screen.getByRole('log', { name: 'Conversation' })).toHaveAttribute('aria-live', 'polite')
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText('Partner')).toBeInTheDocument()
    expect(screen.getByText('Me')).toBeInTheDocument()
    const input = screen.getByRole('textbox', { name: 'Reply' })
    const button = screen.getByRole('button', { name: 'Send' })
    expect(input).toHaveAttribute('rows', '1')
    expect(input).toHaveClass('min-h-11', 'min-w-0')
    expect(button).toHaveClass('min-h-11', 'min-w-11')
    expect(input.closest('form')).toHaveClass('sticky', 'bottom-0')
    expect(button).toBeDisabled()
    const user = userEvent.setup()
    await user.type(input, 'One line.{Enter}Another line.')
    expect(send).not.toHaveBeenCalled()
    await user.click(button)
    expect(send).toHaveBeenCalledWith('One line.\nAnother line.')
  })

  it('retains the reply and attachment while a submission is pending', () => {
    render(
      <ChatComposer
        value="My reply"
        onChange={vi.fn()}
        onSubmit={vi.fn()}
        label="Reply"
        sendLabel="Send"
        pending
        attachment={<p>Selected attachment</p>}
        feedback={<p role="alert">Please retry</p>}
      />,
    )
    expect(screen.getByRole('textbox', { name: 'Reply' })).toHaveValue('My reply')
    expect(screen.getByRole('textbox', { name: 'Reply' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    expect(screen.getByText('Selected attachment')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('Please retry')
  })
})
