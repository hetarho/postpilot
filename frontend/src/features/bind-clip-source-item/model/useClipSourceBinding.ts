import { useTranslation } from 'react-i18next'
import {
  emptyCompositionInputs,
  projectCompositionDocument,
  projectDraft,
  type ClipProject,
  type ClipSourceAssociation,
  useClipProjectMutations,
} from '@/entities/clip-project'

interface BoundSource {
  sourceId: string
  fingerprint: string
  durationMs?: number
}

/** Binding one whole selected source to one item of a declared group, before
 *  any generation (CLIP-123). The range is the source's own `[0, durationMs)`,
 *  so every cut taken from it is covered completely and inherits the item
 *  through the existing owner-range rule.
 *
 *  A project that declares no group offers no items, and the control is then
 *  absent rather than empty. */
export function useClipSourceBinding(
  ownerId: string,
  project: ClipProject,
  selected: readonly { fingerprint: string; sourceId?: string }[] = [],
) {
  const { t } = useTranslation('clips')
  const { save } = useClipProjectMutations(ownerId)
  const document = projectCompositionDocument(project.composition?.snapshot.body)
  const inputs = project.compositionInputs ?? emptyCompositionInputs()
  const items = (document?.groups ?? []).flatMap((group) =>
    (inputs.items[group.id] ?? []).map((item, index) => ({
      value: `${group.id}/${item.id}`,
      label:
        [group.label, Object.values(item.values).filter(Boolean).join(' · ')]
          .filter(Boolean)
          .join(' · ') || t('source.boundItemNumber', { n: index + 1 }),
    })),
  )
  // Only a binding covering the whole source is this control's; a range bound
  // beside the preview keeps its own extent and is left alone here.
  const whole = (a: ClipSourceAssociation, source: BoundSource) =>
    a.sourceId === source.sourceId && a.fingerprint === source.fingerprint && a.startMs === 0
  // A whole-source binding whose source is no longer selected — replaced or
  // removed — is dropped on the next write rather than kept for a source that
  // cannot produce a cut (CLIP-123). Ranges bound beside the preview are left
  // alone; only this control's own bindings are pruned.
  const stale = (a: ClipSourceAssociation) =>
    a.startMs === 0 &&
    !selected.some((s) => s.sourceId === a.sourceId && s.fingerprint === a.fingerprint)
  return {
    items,
    value: (source: BoundSource) => {
      const bound = inputs.associations.find((a) => whole(a, source))
      return bound ? `${bound.groupId}/${bound.itemId}` : ''
    },
    change: (source: BoundSource, item: string) => {
      const [groupId, itemId] = item.split('/')
      const kept = inputs.associations.filter((a) => !whole(a, source) && !stale(a))
      const associations =
        item && groupId && itemId && source.durationMs
          ? [
              ...kept,
              {
                groupId,
                itemId,
                sourceId: source.sourceId,
                fingerprint: source.fingerprint,
                startMs: 0,
                endMs: source.durationMs,
              },
            ]
          : kept
      void save.mutateAsync({
        id: project.id,
        draft: { ...projectDraft(project), compositionInputs: { ...inputs, associations } },
      })
    },
    pending: save.isPending,
    failure: save.error,
  }
}
