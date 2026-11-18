import { useState } from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { withProviders, createTestQueryClient } from '@/test/session'
import { registerTemplateService } from '@/test/templates'
import { afterEach, expect, it, vi } from 'vitest'
import { TemplateDirectEditor } from './TemplateDirectEditor'

afterEach(cleanup)

// The shape of a template is made by talking to the AI; by hand, the owner edits its source text.
// There is no block builder — nothing to drag, move or reorder — and no 고급 to open first.
it('shows the source at once with no builder, and round-trips an edit', async () => {
  const onChange = vi.fn()
  function Editor() {
    const [source, setSource] = useState({
      id: 'draft',
      name: 'Review',
      description: '',
      titleArea: '<write>Title</write>',
      body: '  고정 문구\n<write>Visit</write>  ',
      builderState: '',
    })
    return (
      <TemplateDirectEditor
        source={source}
        disabled={false}
        onChange={(next) => {
          onChange(next)
          setSource({ ...next, builderState: next.builderState ?? '' })
        }}
      />
    )
  }
  const transport = createRouterTransport((router) => registerTemplateService(router))
  render(<Editor />, { wrapper: withProviders(transport, createTestQueryClient()) })
  expect(screen.queryByRole('button', { name: '고급' })).not.toBeInTheDocument()
  expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /옮기기|위로|아래로/ })).not.toBeInTheDocument()
  expect(screen.getByLabelText('원문')).toHaveValue('  고정 문구\n<write>Visit</write>  ')
  expect(onChange).not.toHaveBeenCalled()
  const edited = '  고정 문구\n<write>Changed</write>  '
  fireEvent.change(screen.getByLabelText('원문'), { target: { value: edited } })
  expect(screen.getByLabelText('원문')).toHaveValue(edited)
  expect(onChange).toHaveBeenCalledTimes(1)
})

it.each(['null', '[]', 'true', '42', '"text"', '{'])(
  'opens canonical source without a write when builder metadata is %s',
  (builderState) => {
    const onChange = vi.fn()
    const transport = createRouterTransport((router) => registerTemplateService(router))
    render(
      <TemplateDirectEditor
        source={{
          id: 'draft',
          name: 'Review',
          description: '',
          titleArea: '<write>Title</write>',
          body: '<write>Visit</write>',
          builderState,
        }}
        onChange={onChange}
        disabled={false}
      />,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )
    expect(screen.getByDisplayValue('Review')).toBeInTheDocument()
    expect(onChange).not.toHaveBeenCalled()
  },
)
