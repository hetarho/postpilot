import {
  parseClipComposition,
  type ClipComposition,
} from '@/entities/clip-template/@x/clip-project'
import { CLIP_COMPOSITION_LIMITS } from '@/entities/clip-design/@x/clip-project'
import type { ClipCompositionInputs } from './composition'

export const emptyCompositionInputs = (): ClipCompositionInputs => ({
  values: {},
  items: {},
  associations: [],
})

/** Display missing minimum items without changing the saved draft. The number
 * opened is the group's effective minimum (CLIP-119), not its declared one, so
 * a group holding a required field opens an item to carry it. Stable local IDs
 * become real item IDs only when the owner edits the inputs. */
export function compositionInputsAtMinimum(
  document: ClipComposition,
  input: ClipCompositionInputs,
): ClipCompositionInputs {
  const items = { ...input.items }
  for (const group of document.groups) {
    const minimum = document.minima[group.id] ?? group.min
    const stored = items[group.id] ?? []
    if (stored.length >= minimum) continue
    const shown = [...stored]
    const ids = new Set(stored.map((item) => item.id))
    for (let n = 1; shown.length < minimum; n++) {
      const id = `minimum_item_${n}`
      if (!ids.has(id)) shown.push({ id, values: {} })
    }
    items[group.id] = shown
  }
  return { ...input, items }
}

/** Project only IDs still declared by the same template; never match labels or item offsets. */
export function matchingCompositionInputs(
  document: ClipComposition,
  input: ClipCompositionInputs,
): ClipCompositionInputs {
  const values = (group: string, source: Record<string, string>) =>
    Object.fromEntries(
      document.fields
        .filter((f) => f.group === group && Object.hasOwn(source, f.id))
        .map((f) => [f.id, source[f.id]]),
    )
  const items = Object.fromEntries(
    document.groups
      .filter((g) => Object.hasOwn(input.items, g.id))
      .map(({ id: group }) => [
        group,
        input.items[group].map((item) => ({ id: item.id, values: values(group, item.values) })),
      ]),
  )
  return {
    values: values('', input.values),
    items,
    associations: input.associations.filter((a) =>
      items[a.groupId]?.some((item) => item.id === a.itemId),
    ),
  }
}

export function projectCompositionDocument(body: string | undefined): ClipComposition | undefined {
  if (!body) return undefined
  try {
    return parseClipComposition(body)
  } catch {
    return undefined
  }
}

export function validCompositionInputs(document: ClipComposition, input: ClipCompositionInputs) {
  const validValues = (values: Record<string, string>, group: string) => {
    const fields = document.fields.filter((f) => f.group === group)
    return (
      Object.keys(values).every(
        (id) =>
          fields.some((f) => f.id === id) &&
          Array.from(values[id]).length <= CLIP_COMPOSITION_LIMITS.answerChars,
      ) && fields.every((f) => !f.required || !!values[f.id]?.trim())
    )
  }
  return (
    validValues(input.values, '') &&
    Object.entries(input.items).every(
      ([group, items]) =>
        document.groups.some((g) => g.id === group && items.length <= g.max) &&
        items.length <= CLIP_COMPOSITION_LIMITS.items &&
        new Set(items.map((item) => item.id)).size === items.length &&
        items.every(
          (item) =>
            /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/.test(item.id) && validValues(item.values, group),
        ),
    )
  )
}

export function removeCompositionItem(
  input: ClipCompositionInputs,
  group: string,
  id: string,
): ClipCompositionInputs {
  return {
    ...input,
    items: { ...input.items, [group]: (input.items[group] ?? []).filter((item) => item.id !== id) },
    associations: input.associations.filter((a) => a.groupId !== group || a.itemId !== id),
  }
}
