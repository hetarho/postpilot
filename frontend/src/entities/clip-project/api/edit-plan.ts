import {
  CLIP_ACCENTS,
  COPY_STYLES,
  type ClipAccent,
  type CopyStyle,
} from '@/entities/clip-template/@x/clip-project'
import type { ProtoClipEditingState } from '@/shared/api'
import {
  COPY_ALIGNS,
  COPY_ANCHORS,
  type ClipCaption,
  type ClipEditingState,
  type ClipEditPlan,
} from '../model/edit-plan'

export function toClipEditingState(value: ProtoClipEditingState): ClipEditingState {
  if (!value.plan) throw new Error('Missing clip edit plan')
  if (value.copyStyles.some((s) => !COPY_STYLES.includes(s as CopyStyle)))
    throw new Error('Invalid approved styles')
  return {
    plan: {
      ...(value.plan.nativeComposition
        ? {
            nativeComposition: true,
            associations:
              value.plan.associations?.values.map((a) => ({
                groupId: a.groupId,
                itemId: a.itemId,
                sourceId: a.sourceId,
                fingerprint: a.fingerprint,
                startMs: a.startMs,
                endMs: a.endMs,
              })) ?? [],
            elements: value.plan.elements.map(({ $typeName, rows, phrases, evidence, ...text }) => {
              void $typeName
              return {
                ...text,
                phrases: phrases.map((p) => ({ text: p.text, startMs: p.startMs, endMs: p.endMs })),
                evidence: evidence.map((e) => ({
                  sourceId: e.sourceId,
                  fingerprint: e.fingerprint,
                  startMs: e.startMs,
                  endMs: e.endMs,
                })),
                rows: rows.map((row) => ({ role: row.role, text: row.text })),
              }
            }),
          }
        : {}),
      durationMs: value.plan.durationMs,
      hook: value.plan.hook,
      cuts: value.plan.cuts.map((c) => {
        // `copies` is the authority; a server that still sends only the one
        // `copy` is read exactly as it was before CDS-43.
        const wire =
          c.copies.length > 0
            ? c.copies
            : [
                c.copy ?? {
                  text: '',
                  pace: '',
                  position: '',
                  align: '',
                  style: '',
                  accent: '',
                  keyword: '',
                  startMs: 0,
                  endMs: 0,
                },
              ]
        // A dropped caption has no wire entry. Supply an empty editable slot;
        // saving it still renders only the footage until the owner types text.
        const copies = wire.map((copy) => {
          // A cut whose copy the composer dropped arrives with no placement:
          // valid, and the correction screen shows it as a cut with no text.
          const placed = copy.text.trim() !== ''
          if (
            !['', 'steady', 'rapid'].includes(copy.pace) ||
            (placed &&
              (!COPY_ANCHORS.includes(copy.position as ClipCaption['anchor']) ||
                !COPY_ALIGNS.includes(copy.align as ClipCaption['align']) ||
                !COPY_STYLES.includes(copy.style as CopyStyle))) ||
            !CLIP_ACCENTS.includes(copy.accent as ClipAccent)
          )
            throw new Error('Invalid clip caption')
          return {
            ...(copy.pace ? { pace: copy.pace as NonNullable<ClipCaption['pace']> } : {}),
            text: copy.text,
            startMs: copy.startMs,
            endMs: copy.endMs,
            anchor: (copy.position || 'bottom') as ClipCaption['anchor'],
            align: (copy.align || 'center') as ClipCaption['align'],
            keyword: copy.keyword,
            style: (copy.style || 'clean') as CopyStyle,
            accent: copy.accent as ClipAccent,
          }
        })
        return {
          ...(c.focal ? { focal: { x: c.focal.x, y: c.focal.y } } : {}),
          id: c.id,
          sourceId: c.sourceId,
          fingerprint: c.fingerprint,
          startMs: c.startMs,
          endMs: c.endMs,
          transitionMs: c.transitionMs,
          volumePermille: c.volumePermille,
          chips: [...c.chips],
          copies,
        }
      }),
    },
    sources: value.sources.map((s) => ({
      id: s.id,
      fingerprint: s.fingerprint,
      filename: s.filename,
      durationMs: s.durationMs,
      width: s.width,
      height: s.height,
    })),
    copyStyles: value.copyStyles as CopyStyle[],
    fadeMs: value.fadeMs,
    maxCuts: value.maxCuts,
    maxCopyRunes: value.maxCopyRunes,
    minDurationMs: value.minDurationMs,
    maxDurationMs: value.maxDurationMs,
  }
}
export function clipPlanToProto(plan: ClipEditPlan) {
  return {
    nativeComposition: plan.nativeComposition ?? false,
    associations: plan.associations ? { values: plan.associations } : undefined,
    elements: plan.elements ?? [],
    durationMs: plan.durationMs,
    hook: plan.hook,
    cuts: plan.cuts.map((c) => ({
      focal: c.focal,
      id: c.id,
      sourceId: c.sourceId,
      fingerprint: c.fingerprint,
      startMs: c.startMs,
      endMs: c.endMs,
      transitionMs: c.transitionMs,
      volumePermille: c.volumePermille,
      chips: [...c.chips],
      // `position` carries the anchor on the wire; the field kept its number
      // through the vocabulary change (CDS-12). `copy` stays populated with the
      // first one for a release, beside the list that is the authority.
      copy: c.copies[0] ? { ...c.copies[0], position: c.copies[0].anchor } : undefined,
      copies: c.copies.map((copy) => ({ ...copy, position: copy.anchor })),
    })),
  }
}
