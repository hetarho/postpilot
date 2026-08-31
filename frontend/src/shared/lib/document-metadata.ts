/** One transactional writer for `<title>`, `<meta>` and `<link rel>` — the only place in the app
 *  that touches document head metadata.
 *
 *  It exists because two writers cannot share the head safely: a route that sets marketing tags
 *  and a provider that sets the app default would each overwrite the other's work in whatever
 *  order their effects happened to run, and neither could put the head back. Every caller goes
 *  through `applyDocumentMetadata`, which records the exact prior state of each tag it touches and
 *  hands back a restore.
 *
 *  It is domain-agnostic on purpose: it owns no copy, reads no catalog, and makes no request.
 *  A caller supplies finished strings. */

/** A tag this transaction may write. `meta` is keyed by `name`, `property` by `property` (the
 *  attribute Open Graph uses), and `link` by `rel`. */
export interface DocumentMetadata {
  title?: string
  meta?: Readonly<Record<string, string>>
  property?: Readonly<Record<string, string>>
  link?: Readonly<Record<string, string>>
}

/** What one tag looked like before this transaction: either it was absent (and must be removed
 *  again) or it held a value (which must be written back verbatim). */
type Previous =
  { element: Element; created: true } | { element: Element; created: false; value: string }

const CONTENT_ATTRIBUTE = 'content'
const HREF_ATTRIBUTE = 'href'

function upsert(
  target: Document,
  selector: string,
  create: () => Element,
  attribute: string,
  value: string,
): Previous {
  const existing = target.head.querySelector(selector)
  if (existing) {
    const previous: Previous = {
      element: existing,
      created: false,
      value: existing.getAttribute(attribute) ?? '',
    }
    existing.setAttribute(attribute, value)
    return previous
  }
  const element = create()
  element.setAttribute(attribute, value)
  target.head.append(element)
  return { element, created: true }
}

function restore(previous: Previous, attribute: string): void {
  if (previous.created) {
    previous.element.remove()
    return
  }
  previous.element.setAttribute(attribute, previous.value)
}

/** Applies the given metadata and returns the exact undo.
 *
 *  The undo is exact in both directions: a tag that already existed keeps its element and gets its
 *  old value back, and a tag this call created is removed. That is what stops one route's cleanup
 *  from deleting another route's metadata, or from leaving its own behind. */
export function applyDocumentMetadata(
  metadata: DocumentMetadata,
  target: Document = document,
): () => void {
  const undo: Array<() => void> = []

  if (metadata.title !== undefined) {
    const previousTitle = target.title
    target.title = metadata.title
    undo.push(() => {
      target.title = previousTitle
    })
  }

  for (const [name, content] of Object.entries(metadata.meta ?? {})) {
    const previous = upsert(
      target,
      `meta[name="${name}"]`,
      () => {
        const element = target.createElement('meta')
        element.setAttribute('name', name)
        return element
      },
      CONTENT_ATTRIBUTE,
      content,
    )
    undo.push(() => restore(previous, CONTENT_ATTRIBUTE))
  }

  for (const [property, content] of Object.entries(metadata.property ?? {})) {
    const previous = upsert(
      target,
      `meta[property="${property}"]`,
      () => {
        const element = target.createElement('meta')
        element.setAttribute('property', property)
        return element
      },
      CONTENT_ATTRIBUTE,
      content,
    )
    undo.push(() => restore(previous, CONTENT_ATTRIBUTE))
  }

  for (const [rel, href] of Object.entries(metadata.link ?? {})) {
    const previous = upsert(
      target,
      `link[rel="${rel}"]`,
      () => {
        const element = target.createElement('link')
        element.setAttribute('rel', rel)
        return element
      },
      HREF_ATTRIBUTE,
      href,
    )
    undo.push(() => restore(previous, HREF_ATTRIBUTE))
  }

  // Reverse order, so two calls that touched the same tag unwind like a stack.
  return () => {
    for (let index = undo.length - 1; index >= 0; index -= 1) undo[index]()
  }
}
