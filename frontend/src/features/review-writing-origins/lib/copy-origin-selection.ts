import type { ClipboardEvent } from 'react'

const SEMANTIC_ELEMENTS = new Set([
  'P',
  'H1',
  'H2',
  'H3',
  'H4',
  'H5',
  'H6',
  'UL',
  'OL',
  'LI',
  'BLOCKQUOTE',
  'BR',
  'STRONG',
  'EM',
  'B',
  'I',
  'CODE',
  'PRE',
  'FIGURE',
  'FIGCAPTION',
])
const OMITTED_ELEMENTS = new Set(['SCRIPT', 'STYLE', 'INPUT', 'TEXTAREA', 'BUTTON', 'IMG', 'VIDEO'])

/** Copy the selected words and semantic structure, never the review DOM's styles,
 *  interaction attributes or private media links. Text nodes remain literal text,
 *  including owner wording that happens to resemble annotation markup. */
function cleanSelectionNode(node: Node, document: Document): Node | null {
  if (node.nodeType === Node.TEXT_NODE) return document.createTextNode(node.textContent ?? '')
  if (node instanceof Element) {
    if (node.hasAttribute('data-writing-origin-review-ui') || OMITTED_ELEMENTS.has(node.tagName))
      return null
    const clean = SEMANTIC_ELEMENTS.has(node.tagName)
      ? document.createElement(node.tagName.toLowerCase())
      : document.createDocumentFragment()
    for (const child of node.childNodes) {
      const result = cleanSelectionNode(child, document)
      if (result) clean.appendChild(result)
    }
    return clean
  }
  const clean = document.createDocumentFragment()
  for (const child of node.childNodes) {
    const result = cleanSelectionNode(child, document)
    if (result) clean.appendChild(result)
  }
  return clean
}

function structuralText(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? ''
  if (node instanceof Element && node.tagName === 'BR') return '\n'
  const text = Array.from(node.childNodes, structuralText).join('')
  if (!(node instanceof Element)) return text
  if (node.tagName === 'LI') return `${text}\n`
  if (/^(P|H[1-6]|BLOCKQUOTE|PRE|UL|OL|FIGURE|FIGCAPTION)$/.test(node.tagName)) return `${text}\n\n`
  return text
}

function renderedSelectionText(clean: HTMLElement): string {
  // Native innerText retains paragraph/list boundaries and authored whitespace.
  // Measure a private offscreen clone, leaving the real selection/focus/caret intact.
  Object.assign(clean.style, {
    position: 'fixed',
    left: '-10000px',
    top: '0',
    whiteSpace: 'pre-wrap',
    pointerEvents: 'none',
  })
  clean.setAttribute('aria-hidden', 'true')
  clean.inert = true
  clean.ownerDocument.body.appendChild(clean)
  try {
    return typeof clean.innerText === 'string'
      ? clean.innerText
      : structuralText(clean).replace(/\n+$/, '')
  } finally {
    clean.remove()
  }
}

export function copyOriginSelection(event: ClipboardEvent<HTMLDivElement>) {
  if (event.defaultPrevented || !event.clipboardData) return
  if (event.target instanceof Element && event.target.closest('input, textarea, [contenteditable]'))
    return
  const root = event.currentTarget
  const selection = root.ownerDocument.getSelection()
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return
  const phrases = root.querySelectorAll('[data-writing-origin-phrase]')
  const ranges = Array.from({ length: selection.rangeCount }, (_, index) =>
    selection.getRangeAt(index),
  )
  // Ordinary copying elsewhere in the editor stays browser-owned. Handle only
  // ranges wholly inside this review's rendered content that touch highlighted prose.
  if (ranges.some((range) => !root.contains(range.commonAncestorContainer))) return
  if (!ranges.some((range) => Array.from(phrases).some((phrase) => range.intersectsNode(phrase))))
    return
  const clean = root.ownerDocument.createElement('div')
  let removedReviewUi = false
  for (const range of ranges) {
    const selected = range.cloneContents()
    removedReviewUi ||= Boolean(selected.querySelector('[data-writing-origin-review-ui]'))
    const fragment = cleanSelectionNode(selected, root.ownerDocument)
    if (fragment) clean.appendChild(fragment)
  }
  event.clipboardData.setData(
    'text/plain',
    removedReviewUi ? renderedSelectionText(clean) : selection.toString(),
  )
  event.clipboardData.setData('text/html', clean.innerHTML)
  event.preventDefault()
}
