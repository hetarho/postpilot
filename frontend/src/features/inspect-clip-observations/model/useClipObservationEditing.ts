import { useCallback, useRef, useState, useSyncExternalStore } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useClipProjectCalls, useClipProjectsKey, type ClipProject } from '@/entities/clip-project'
import type { ClipObservedSegment } from '@/entities/clip-observation'
import type { AppFailure } from '@/shared/api'
import { resolveSaveStatus, SAVE_STATUS_LABEL_KEYS } from '@/shared/lib'
import {
  clipObservationEditFailure,
  clipObservationSaveState,
  flushClipObservationEdits,
  peekClipObservationEdits,
  queueClipObservationEdit,
  subscribeClipObservationEdits,
} from './observation-queue'
import { observationEditOf, type ObservationField } from './observation-edits'

/** What ② needs to correct observed scenes in place (CLIP-235): the text on screen, the change
 *  that autosaves it, the field a correction is refused on, and the queue's flush. */
export interface ClipObservationEditing {
  editable: boolean
  text(observationId: string, field: ObservationField, segment: ClipObservedSegment): string
  change(
    observationId: string,
    field: ObservationField,
    segment: ClipObservedSegment,
    value: string,
  ): void
  /** The field refused locally: observed speech cannot be cleared (CLIP-235, CLIP-129). */
  refused(observationId: string): ObservationField | undefined
  failure: AppFailure | undefined
}

/** The workspace-level owner of ②'s corrections: mounted above the steps so typed text survives
 *  a step change, and read-only while a job runs or the project is finalized (CLIP-160). */
export function useClipObservationEditing(
  ownerId: string,
  project: ClipProject,
  readOnly: boolean,
): ClipObservationEditing & {
  flush(failFast?: boolean): Promise<void>
  hasEdits(sourceId: string): boolean
  status: { failing: boolean; label: string; failure?: AppFailure }
} {
  const { t } = useTranslation('clips')
  const { t: common } = useTranslation('common')
  const calls = useClipProjectCalls()
  const cache = useQueryClient()
  const projectsKey = useClipProjectsKey(ownerId)
  const projectId = project.id
  const subscribe = useCallback(
    (listener: () => void) => subscribeClipObservationEdits(projectId, listener),
    [projectId],
  )
  const owed = useSyncExternalStore(
    subscribe,
    () => peekClipObservationEdits(projectId),
    () => undefined,
  )
  const failure = useSyncExternalStore(
    subscribe,
    () => clipObservationEditFailure(projectId),
    () => undefined,
  )
  const state = useSyncExternalStore(
    subscribe,
    () => clipObservationSaveState(projectId),
    () => 'idle' as const,
  )
  // What the owner typed that the queue cannot take — a cleared speech line — stays on screen
  // with its error instead of snapping back to the stored words.
  const [held, setHeld] = useState<Record<string, { field: ObservationField; value: string }>>({})
  const heldRef = useRef(held)
  const ensureValid = () => {
    if (Object.keys(heldRef.current).length)
      throw {
        reason: 'CLIP_INVALID_INPUT',
        params: { check: 'speech_audibility' },
      } satisfies AppFailure
  }
  const invalid = Object.keys(held).length > 0
  const label = SAVE_STATUS_LABEL_KEYS[resolveSaveStatus(state, true)]
  return {
    editable: !readOnly,
    failure,
    text(observationId, field, segment) {
      const kept = held[observationId]
      if (kept?.field === field) return kept.value
      const edit = owed?.[observationId]
      if (field === 'subjects') return (edit?.subjects ?? segment.subjects).join('\n')
      return edit?.[field] ?? segment[field]
    },
    change(observationId, field, segment, value) {
      if (readOnly) return
      const edit = observationEditOf(observationId, field, segment, value)
      const next = { ...heldRef.current }
      if (!edit) next[observationId] = { field, value }
      else if (next[observationId]?.field === field) delete next[observationId]
      heldRef.current = next
      setHeld(next)
      if (!edit) return
      queueClipObservationEdit(projectId, edit, async (edits) => {
        await calls.saveObservationEdits(projectId, edits)
        await cache.invalidateQueries({ queryKey: projectsKey })
      })
    },
    refused: (observationId) => held[observationId]?.field,
    hasEdits: (sourceId) => Object.keys(owed ?? {}).some((id) => id.startsWith(`${sourceId}/`)),
    status: {
      failing: invalid || !!failure,
      failure,
      label: invalid
        ? t('observation.speechRequired')
        : failure
          ? t('observation.saveFailed')
          : label
            ? common(label)
            : '',
    },
    flush: async (failFast = false) => {
      ensureValid()
      await flushClipObservationEdits(projectId, failFast)
      ensureValid()
    },
  }
}
