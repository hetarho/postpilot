import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import { FieldLabel } from './FieldLabel'
import { FieldMessage } from './FieldMessage'
import { Textarea } from './Textarea'
import { TextField } from './TextField'

describe('field primitives', () => {
  it('keeps label, error, and invalid-field relationships explicit', () => {
    render(
      <>
        <FieldLabel htmlFor="title">제목</FieldLabel>
        <TextField id="title" aria-invalid aria-describedby="title-error" />
        <FieldMessage id="title-error">제목을 확인해 주세요</FieldMessage>
      </>,
    )

    expect(screen.getByRole('textbox', { name: '제목' })).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveAttribute('id', 'title-error')
  })

  it('forwards refs for editor caret handoff', () => {
    const inputRef = createRef<HTMLInputElement>()
    const textareaRef = createRef<HTMLTextAreaElement>()
    render(
      <>
        <TextField ref={inputRef} aria-label="제목" appearance="bare" />
        <Textarea ref={textareaRef} aria-label="메모" appearance="bare" />
      </>,
    )

    expect(inputRef.current).toBe(screen.getByRole('textbox', { name: '제목' }))
    expect(textareaRef.current).toBe(screen.getByRole('textbox', { name: '메모' }))
  })

  it('keeps the recessed well and editor-bare appearances distinct', () => {
    render(
      <>
        <TextField aria-label="웰" />
        <TextField aria-label="베어" appearance="bare" />
        <Textarea aria-label="메모 웰" />
        <Textarea aria-label="메모 베어" appearance="bare" />
      </>,
    )

    expect(screen.getByRole('textbox', { name: '웰' })).toHaveClass('bg-field-bg')
    expect(screen.getByRole('textbox', { name: '베어' })).toHaveClass('bg-transparent')
    expect(screen.getByRole('textbox', { name: '메모 웰' })).toHaveClass('bg-field-bg')
    expect(screen.getByRole('textbox', { name: '메모 베어' })).toHaveClass('bg-transparent')
  })

  it('forwards native field props and disabled semantics', () => {
    render(<TextField aria-label="아이디" autoComplete="username" required disabled />)

    const field = screen.getByRole('textbox', { name: '아이디' })
    expect(field).toBeDisabled()
    expect(field).toBeRequired()
    expect(field).toHaveAttribute('autocomplete', 'username')
  })
})
