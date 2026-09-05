import { afterEach, describe, expect, it } from 'vitest'
import './index.css'

function append<K extends keyof HTMLElementTagNameMap>(tag: K) {
  const element = document.createElement(tag)
  document.body.append(element)
  return element
}

function appendFilePicker(disabled = false) {
  const input = append('input')
  input.type = 'file'
  input.disabled = disabled
  const label = append('label')
  return label
}

function appendLabeledCheckbox(disabled = false) {
  const label = append('label')
  const input = document.createElement('input')
  input.type = 'checkbox'
  input.disabled = disabled
  label.append(input, 'Choice')
  return label
}

afterEach(() => document.body.replaceChildren())

describe('global pointer affordances', () => {
  it.each([
    ['destination link', () => Object.assign(append('a'), { href: '/posts' })],
    ['button', () => append('button')],
    ['select', () => append('select')],
    ['disclosure', () => append('summary')],
    ['checkbox', () => Object.assign(append('input'), { type: 'checkbox' })],
    ['checkbox label', () => appendLabeledCheckbox()],
    ['file-picker label', () => appendFilePicker()],
    [
      'ARIA option',
      () => {
        const element = append('div')
        element.setAttribute('role', 'option')
        return element
      },
    ],
  ])('%s uses the pointer cursor', (_name, create) => {
    expect(getComputedStyle(create()).cursor).toBe('pointer')
  })

  it('does not advertise disabled controls as clickable', () => {
    const button = append('button')
    button.disabled = true
    const ariaButton = append('div')
    ariaButton.setAttribute('role', 'button')
    ariaButton.setAttribute('aria-disabled', 'true')
    const filePickerLabel = appendFilePicker(true)
    const checkboxLabel = appendLabeledCheckbox(true)

    expect(getComputedStyle(button).cursor).toBe('default')
    expect(getComputedStyle(ariaButton).cursor).toBe('default')
    expect(getComputedStyle(filePickerLabel).cursor).toBe('default')
    expect(getComputedStyle(checkboxLabel).cursor).toBe('default')
  })

  it('leaves text editing and a specialized drag cursor intact', () => {
    const input = append('input')
    input.type = 'text'
    const textarea = append('textarea')
    const dragHandle = append('span')
    dragHandle.style.cursor = 'grab'

    expect(getComputedStyle(input).cursor).toBe('text')
    expect(getComputedStyle(textarea).cursor).toBe('text')
    expect(getComputedStyle(dragHandle).cursor).toBe('grab')
  })
})
