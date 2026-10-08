import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { TemplateDirectEditor } from './TemplateDirectEditor'

afterEach(cleanup)

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
