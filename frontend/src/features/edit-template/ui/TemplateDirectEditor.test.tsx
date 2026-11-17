import { useState } from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { withProviders, createTestQueryClient } from '@/test/session'
import { registerTemplateService } from '@/test/templates'
import { afterEach, expect, it, vi } from 'vitest'
import { TemplateDirectEditor } from './TemplateDirectEditor'

afterEach(cleanup)

// Templates are made by talking to the AI (owner 2026-11-17): editing by hand is the 원문 text
// alone, with no block builder to add, drag or reorder rows in.
it('edits the template as 원문 alone, with no builder, and round-trips the source', async () => {
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
  expect(screen.getByLabelText('원문')).toHaveValue('  고정 문구\n<write>Visit</write>  ')
  expect(screen.getByLabelText('제목 원문')).toHaveValue('<write>Title</write>')
  expect(screen.getByText('형식 안내 보기')).toBeInTheDocument()
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
