import { useState } from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRouterTransport } from '@connectrpc/connect'
import { withProviders, createTestQueryClient } from '@/test/session'
import { registerTemplateService } from '@/test/templates'
import { afterEach, expect, it, vi } from 'vitest'
import { TemplateDirectEditor } from './TemplateDirectEditor'

afterEach(cleanup)

it('keeps 원문 behind 고급 and round-trips edited source without a mode-switch write', async () => {
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
  const user = userEvent.setup()
  expect(screen.queryByRole('tab', { name: '원문' })).not.toBeInTheDocument()
  expect(screen.queryByLabelText('원문')).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '고급' }))
  await user.click(screen.getByRole('tab', { name: '원문' }))
  expect(screen.getByLabelText('원문')).toHaveValue('  고정 문구\n<write>Visit</write>  ')
  expect(screen.getByText('형식 안내 보기')).toBeInTheDocument()
  expect(onChange).not.toHaveBeenCalled()
  const edited = '  고정 문구\n<write>Changed</write>  '
  fireEvent.change(screen.getByLabelText('원문'), { target: { value: edited } })
  await user.click(screen.getByRole('button', { name: '고급' }))
  expect(screen.queryByLabelText('원문')).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '고급' }))
  await user.click(screen.getByRole('tab', { name: '원문' }))
  expect(screen.getByLabelText('원문')).toHaveValue(edited)
  expect(onChange).toHaveBeenCalledTimes(1)
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
    )
    expect(screen.getByDisplayValue('Review')).toBeInTheDocument()
    expect(onChange).not.toHaveBeenCalled()
  },
)
