import { useState } from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { withProviders, createTestQueryClient } from '@/test/session'
import { registerTemplateService } from '@/test/templates'
import { afterEach, expect, it, vi } from 'vitest'
import { TemplateDirectEditor } from './TemplateDirectEditor'

afterEach(cleanup)

it('edits the template as its own text, byte for byte, with no builder to drag', async () => {
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
  // No mode switch and no block rows: the source is the editor.
  expect(screen.queryByRole('button', { name: '고급' })).not.toBeInTheDocument()
  expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
  expect(screen.queryAllByRole('button', { name: /위로|아래로|끌어서/ })).toEqual([])
  expect(screen.getByLabelText('원문')).toHaveValue('  고정 문구\n<write>Visit</write>  ')
  expect(screen.getByLabelText('제목 원문')).toHaveValue('<write>Title</write>')
  expect(onChange).not.toHaveBeenCalled()
  const edited = '  고정 문구\n<write>Changed</write>  '
  fireEvent.change(screen.getByLabelText('원문'), { target: { value: edited } })
  expect(screen.getByLabelText('원문')).toHaveValue(edited)
  expect(onChange).toHaveBeenCalledTimes(1)
  expect(onChange.mock.calls[0][0]).toMatchObject({ body: edited })
})

it.each(['null', '[]', 'true', '42', '"text"', '{'])(
  'opens canonical source without a write when builder metadata is %s',
  (builderState) => {
    const onChange = vi.fn()
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
      {
        wrapper: withProviders(
          createRouterTransport((router) => registerTemplateService(router)),
          createTestQueryClient(),
        ),
      },
    )
    expect(screen.getByDisplayValue('Review')).toBeInTheDocument()
    expect(onChange).not.toHaveBeenCalled()
  },
)
