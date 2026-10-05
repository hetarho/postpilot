import type { ClipEditPlan } from './edit-plan'

/** Display wording alone. The server repeats this rune partition before admitting refresh. */
export function captionWords(text: string, count: number) {
  const runes = Array.from(text)
  let start = 0
  return Array.from({ length: count }, (_, i) => {
    let end = Math.floor((runes.length * (i + 1)) / count)
    if (i < count - 1) {
      for (let j = end; j < runes.length; j++) {
        if (/^\p{White_Space}$/u.test(runes[j])) {
          end = j
          break
        }
      }
    }
    end = Math.max(start, end)
    const words = runes
      .slice(start, end)
      .join('')
      .replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '')
    start = end
    return words
  })
}
export function captionRefresh(plan: ClipEditPlan) {
  const wording = new Map<string, { text: string; textRevision: number }>()
  const pending = [] as NonNullable<ClipEditPlan['narration']>['segments']
  const orphaned: string[] = []
  const unplaceable: string[] = []
  const segments = plan.narration?.segments ?? []
  for (const text of plan.elements ?? [])
    if (text.derivedCaption && !segments.some((s) => s.id === text.derivedCaption!.segmentId))
      orphaned.push(text.instanceId)
  for (const segment of segments) {
    const captions = (plan.elements ?? [])
      .filter((t) => t.derivedCaption?.segmentId === segment.id)
      .sort((a, b) => a.resolvedStartMs - b.resolvedStartMs)
    if (!captions.length) {
      pending.push(segment)
      continue
    }
    const words = captionWords(segment.text, captions.length)
    captions.forEach((text, i) => {
      if (
        !text.derivedCaption!.textEdited &&
        text.derivedCaption!.textRevision !== segment.textRevision &&
        text.phrases?.length &&
        captionWords(words[i], text.phrases.length).some((p) => !p)
      ) {
        unplaceable.push(text.instanceId)
        return
      }
      if (
        !text.derivedCaption!.textEdited &&
        text.derivedCaption!.textRevision !== segment.textRevision &&
        !words[i]
      )
        unplaceable.push(text.instanceId)
      if (
        !text.derivedCaption!.textEdited &&
        text.derivedCaption!.textRevision !== segment.textRevision &&
        words[i]
      )
        wording.set(text.instanceId, { text: words[i], textRevision: segment.textRevision })
    })
  }
  return { wording, pending, orphaned, unplaceable }
}
export function refreshCaptionWording(plan: ClipEditPlan): ClipEditPlan {
  const { wording } = captionRefresh(plan)
  if (!wording.size) return plan
  return {
    ...plan,
    refreshDerivedCaptions: true,
    elements: plan.elements?.map((text) => {
      const update = wording.get(text.instanceId)
      return update
        ? {
            ...text,
            text: update.text,
            phrases: text.phrases?.length
              ? text.phrases.map((phrase, i) => ({
                  ...phrase,
                  text: captionWords(update.text, text.phrases!.length)[i],
                }))
              : [],
            ownerEdited: true,
            derivedCaption: { ...text.derivedCaption!, textRevision: update.textRevision },
          }
        : text
    }),
  }
}
