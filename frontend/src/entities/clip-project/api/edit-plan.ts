import {
  CLIP_ACCENTS,
  COPY_STYLES,
  type ClipAccent,
  type CopyStyle,
} from '@/entities/clip-template/@x/clip-project'
import type { ProtoClipEditingState } from '@/shared/api'
import {
  COPY_POSITIONS,
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
      durationMs: value.plan.durationMs,
      cuts: value.plan.cuts.map((c) => {
        const copy = c.copy
        if (
          !copy ||
          !COPY_POSITIONS.includes(copy.position as ClipCaption['position']) ||
          !COPY_STYLES.includes(copy.style as CopyStyle) ||
          !CLIP_ACCENTS.includes(copy.accent as ClipAccent)
        )
          throw new Error('Invalid clip caption')
        return {
          id: c.id,
          sourceId: c.sourceId,
          fingerprint: c.fingerprint,
          startMs: c.startMs,
          endMs: c.endMs,
          volumePermille: c.volumePermille,
          copy: {
            text: copy.text,
            startMs: copy.startMs,
            endMs: copy.endMs,
            position: copy.position as ClipCaption['position'],
            style: copy.style as CopyStyle,
            accent: copy.accent as ClipAccent,
          },
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
    durationMs: plan.durationMs,
    cuts: plan.cuts.map((c) => ({
      id: c.id,
      sourceId: c.sourceId,
      fingerprint: c.fingerprint,
      startMs: c.startMs,
      endMs: c.endMs,
      volumePermille: c.volumePermille,
      copy: { ...c.copy },
    })),
  }
}
