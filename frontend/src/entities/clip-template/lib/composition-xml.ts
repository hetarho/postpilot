import {
  CompositionProblem,
  type CompositionLimits,
  type CompositionNode,
  type CompositionReason,
} from '../model/composition'

export function problem(n: CompositionNode, reason: CompositionReason): never {
  throw new CompositionProblem(n.attributes.id || n.name, n.span.line, reason)
}
export const scalarLength = (s: string) => Array.from(s).length
const white = (c: string) => c === ' ' || c === '\n' || c === '\r' || c === '\t'
export function xmlScalar(c: number) {
  return (
    c === 9 ||
    c === 10 ||
    c === 13 ||
    (c >= 32 && c <= 0xd7ff) ||
    (c >= 0xe000 && c <= 0xfffd) ||
    (c >= 0x10000 && c <= 0x10ffff)
  )
}
function decode(source: string, n: CompositionNode): string {
  let output = ''
  for (let i = 0; i < source.length;) {
    if (source[i] !== '&') {
      const c = source.codePointAt(i)!
      if (!xmlScalar(c)) problem(n, 'invalid_entity')
      output += String.fromCodePoint(c)
      i += c > 0xffff ? 2 : 1
      continue
    }
    const end = source.indexOf(';', i)
    if (end < 0) problem(n, 'invalid_entity')
    const entity = source.slice(i + 1, end)
    const named = new Map([
      ['amp', '&'],
      ['lt', '<'],
      ['gt', '>'],
      ['quot', '"'],
      ['apos', "'"],
    ])
    if (named.has(entity)) output += named.get(entity)
    else {
      if (!/^#(?:[0-9]+|x[0-9a-fA-F]+)$/.test(entity)) problem(n, 'invalid_entity')
      const value = entity.startsWith('#x')
        ? Number.parseInt(entity.slice(2), 16)
        : Number.parseInt(entity.slice(1), 10)
      if (!Number.isSafeInteger(value) || !xmlScalar(value)) problem(n, 'invalid_entity')
      output += String.fromCodePoint(value)
    }
    i = end + 1
  }
  return output
}

/** A bounded XML subset. No DOM, DTD, entities beyond XML scalars, or network. */
export function readCompositionXML(source: string, limits: CompositionLimits): CompositionNode {
  const chars = Array.from(source)
  let at = 0,
    count = 0
  const line = (end: number) => 1 + chars.slice(0, end).filter((c) => c === '\n').length
  const has = (s: string) => chars.slice(at, at + s.length).join('') === s
  const skip = () => {
    while (at < chars.length && white(chars[at])) at++
  }
  const name = () => {
    const start = at
    while (at < chars.length && (at === start ? /[a-zA-Z_]/ : /[a-zA-Z0-9_-]/).test(chars[at])) at++
    return chars.slice(start, at).join('')
  }
  const read = (): CompositionNode => {
    const start = at,
      startLine = line(start)
    if (!has('<')) throw new CompositionProblem('clip', startLine, 'syntax')
    at++
    const tag = name()
    if (!tag) throw new CompositionProblem('clip', line(at), 'unsafe_construct')
    const node: CompositionNode = {
      name: tag,
      attributes: {},
      children: [],
      text: '',
      span: { start, end: 0, line: startLine },
    }
    if (++count > limits.nodes) problem(node, 'node_limit')
    for (;;) {
      const before = at
      skip()
      if (has('/>')) {
        at += 2
        node.span.end = at
        return node
      }
      if (has('>')) {
        at++
        break
      }
      if (before === at) problem(node, 'syntax')
      const key = name()
      if (!key) problem(node, 'syntax')
      if (Object.hasOwn(node.attributes, key)) problem(node, 'duplicate_attribute')
      skip()
      if (!has('=')) problem(node, 'syntax')
      at++
      skip()
      const quote = chars[at]
      if (quote !== "'" && quote !== '"') problem(node, 'syntax')
      at++
      const a = at
      while (at < chars.length && chars[at] !== quote) {
        if (chars[at] === '<') problem(node, 'syntax')
        at++
      }
      if (at === chars.length) problem(node, 'syntax')
      Object.defineProperty(node.attributes, key, {
        value: decode(chars.slice(a, at).join(''), node),
        enumerable: true,
        configurable: true,
        writable: true,
      })
      at++
    }
    while (at < chars.length) {
      if (has('</')) {
        at += 2
        const close = name()
        skip()
        if (close !== tag || !has('>')) problem(node, 'syntax')
        at++
        node.span.end = at
        return node
      }
      if (has('<')) {
        node.children.push(read())
        continue
      }
      const a = at
      while (at < chars.length && chars[at] !== '<') at++
      const raw = chars.slice(a, at).join(''),
        text = decode(raw, node)
      if (raw.includes(']]>')) problem(node, 'invalid_entity')
      node.children.push({
        name: '#text',
        attributes: {},
        children: [],
        text,
        span: { start: a, end: at, line: line(a) },
      })
    }
    return problem(node, 'syntax')
  }
  skip()
  const root = read()
  skip()
  if (at !== chars.length) problem(root, 'syntax')
  return root
}

function escapeXML(s: string, attribute = false) {
  const escaped = s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
  return attribute ? escaped.replaceAll('"', '&quot;') : escaped
}
export function serializeCompositionNode(n: CompositionNode): string {
  if (n.name === '#text') return escapeXML(n.text)
  const attrs = Object.keys(n.attributes)
    .sort()
    .map((key) => ` ${key}="${escapeXML(n.attributes[key], true)}"`)
    .join('')
  return n.children.length
    ? `<${n.name}${attrs}>${n.children.map(serializeCompositionNode).join('')}</${n.name}>`
    : `<${n.name}${attrs}/>`
}
