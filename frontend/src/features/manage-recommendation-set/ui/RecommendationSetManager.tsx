import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  MAX_RECOMMENDATION_SETS,
  RECOMMENDATION_STAGES,
  STAGE_PURPOSE,
  recommendationSlots,
  sameRef,
  useAdminCatalog,
  useDeleteRecommendationSet,
  useModels,
  useMoveRecommendationSet,
  useRecommendationSets,
  type AdminCatalogEntry,
  type ModelRef,
  type RecommendationSet,
  type StageName,
} from '@/entities/model-catalog'
import { AppFailureMessage, Badge, Button, Dialog, Notice, Typography } from '@/shared/ui'
import { RecommendationSetEditor } from './RecommendationSetEditor'

const NEW_SET: RecommendationSet = { id: '', label: '', selections: [] }

type SlotFlag = 'unregistered' | 'unclassified'

/** The 추천 조합 tab (MODEL-69): the operator's ordered list of the sets every account is offered,
 *  each editable, movable and deletable, plus one editor at a time.
 *
 *  A set is advice (MODEL-71): nothing here reaches an account's own selections, which is why
 *  a delete needs a confirmation but no warning about users losing their choice.
 *
 *  A slot whose model has since been deregistered or declassified keeps its value — a saved set
 *  is never edited behind the operator's back (MODEL-70) — and is flagged from the same catalog
 *  reads the 모델 관리 tab makes. */
export function RecommendationSetManager() {
  const { t } = useTranslation('models')
  const titleId = useId()
  const { sets, isPending, isError } = useRecommendationSets()
  const move = useMoveRecommendationSet()
  const remove = useDeleteRecommendationSet()
  const flags = useSlotFlags()
  const [editing, setEditing] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<RecommendationSet | null>(null)
  const full = sets.length >= MAX_RECOMMENDATION_SETS
  const writeFailure = move.failure ?? remove.failure

  return (
    <section aria-labelledby={titleId} className="mt-8 grid gap-4">
      <div className="grid gap-1">
        <Typography variant="title" as="h2" id={titleId}>
          {t('recommendationSets.title')}
        </Typography>
        <Typography variant="body" className="text-content-secondary max-w-measure">
          {t('recommendationSets.description')}
        </Typography>
      </div>

      {isError && (
        <Notice tone="danger" role="alert">
          {t('recommendationSets.loadFailed')}
        </Notice>
      )}
      {writeFailure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={writeFailure} />
        </Notice>
      )}

      {!isError && (
        <div className="grid gap-2">
          <div>
            <Button
              variant="secondary"
              className="w-full sm:w-auto"
              disabled={isPending || full || editing !== null}
              onClick={() => setEditing('')}
            >
              {t('recommendationSets.add')}
            </Button>
          </div>
          {full && (
            <Typography variant="meta" className="text-content-tertiary block">
              {t('recommendationSets.full', { limit: MAX_RECOMMENDATION_SETS })}
            </Typography>
          )}
        </div>
      )}

      {editing === '' && (
        <RecommendationSetEditor initial={NEW_SET} onDone={() => setEditing(null)} />
      )}

      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary">
          {t('recommendationSets.loading')}
        </Typography>
      )}
      {!isError && !isPending && sets.length === 0 && editing !== '' && (
        <Typography variant="body" className="text-content-tertiary">
          {t('recommendationSets.empty')}
        </Typography>
      )}

      {sets.length > 0 && (
        <ol className="grid gap-4">
          {sets.map((set, index) => (
            <li key={set.id}>
              {editing === set.id ? (
                <RecommendationSetEditor initial={set} onDone={() => setEditing(null)} />
              ) : (
                <SetCard
                  set={set}
                  flagOf={flags}
                  first={index === 0}
                  last={index === sets.length - 1}
                  busy={editing !== null || move.isPending || remove.isPending}
                  onEdit={() => setEditing(set.id)}
                  onMove={(earlier) => move.move(set.id, earlier)}
                  onDelete={() => setDeleting(set)}
                />
              )}
            </li>
          ))}
        </ol>
      )}

      <Dialog
        open={deleting !== null}
        title={t('recommendationSets.deleteTitle')}
        confirmLabel={t('recommendationSets.deleteConfirm')}
        pending={remove.isPending}
        onClose={() => setDeleting(null)}
        onConfirm={() => {
          if (!deleting) return
          void remove
            .remove(deleting.id)
            .catch(() => {
              // The mutation state carries the structured failure rendered above.
            })
            .finally(() => setDeleting(null))
        }}
      >
        {t('recommendationSets.deleteBody', { label: deleting?.label ?? '' })}
      </Dialog>
    </section>
  )
}

function SetCard({
  set,
  flagOf,
  first,
  last,
  busy,
  onEdit,
  onMove,
  onDelete,
}: {
  set: RecommendationSet
  flagOf: SlotFlagReader
  first: boolean
  last: boolean
  busy: boolean
  onEdit: () => void
  onMove: (earlier: boolean) => void
  onDelete: () => void
}) {
  const { t } = useTranslation('models')
  const headingId = useId()
  return (
    // A labelled group rather than a nested list: the set's name titles the slots inside it,
    // and the page's one list is the ordered sets.
    <div role="group" aria-labelledby={headingId} className="bg-surface-raised rounded-lg p-4">
      <Typography variant="fieldTitle" as="h3" id={headingId} className="break-words">
        {set.label}
      </Typography>
      <div className="mt-3 grid gap-3 sm:grid-cols-3">
        {RECOMMENDATION_STAGES.map((stage) => {
          const selection = set.selections.find((entry) => entry.stage === stage)
          return (
            <div key={stage} className="min-w-0">
              <Typography variant="label" as="p" className="text-content-secondary">
                {t(`recommendationSets.stageGroup.${stage}`)}
              </Typography>
              <dl className="mt-1 grid gap-1">
                {recommendationSlots(stage).map((slot) => (
                  <SlotLine
                    key={slot}
                    stage={stage}
                    slotLabel={t(`recommendationSets.slot.${slot}`)}
                    modelRef={selection?.[slot]}
                    flagOf={flagOf}
                  />
                ))}
              </dl>
            </div>
          )
        })}
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <Button
          size="compact"
          variant="secondary"
          disabled={busy}
          aria-label={t('recommendationSets.editLabel', { label: set.label })}
          onClick={onEdit}
        >
          {t('recommendationSets.edit')}
        </Button>
        <Button
          size="compact"
          variant="ghost"
          disabled={busy || first}
          aria-label={t('recommendationSets.upLabel', { label: set.label })}
          onClick={() => onMove(true)}
        >
          {t('recommendationSets.up')}
        </Button>
        <Button
          size="compact"
          variant="ghost"
          disabled={busy || last}
          aria-label={t('recommendationSets.downLabel', { label: set.label })}
          onClick={() => onMove(false)}
        >
          {t('recommendationSets.down')}
        </Button>
        <Button
          size="compact"
          variant="danger"
          disabled={busy}
          aria-label={t('recommendationSets.deleteLabel', { label: set.label })}
          onClick={onDelete}
        >
          {t('recommendationSets.delete')}
        </Button>
      </div>
    </div>
  )
}

function SlotLine({
  stage,
  slotLabel,
  modelRef,
  flagOf,
}: {
  stage: StageName
  slotLabel: string
  modelRef: ModelRef | undefined
  flagOf: SlotFlagReader
}) {
  const { t } = useTranslation('models')
  const { label, level, flag } = modelRef ? flagOf(stage, modelRef) : EMPTY_SLOT
  return (
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1">
      <Typography variant="meta" as="dt" className="shrink-0">
        {slotLabel}
      </Typography>
      {/* `break-all`: a model id is a server-supplied slug with no natural break (THEME-21). */}
      <Typography variant="body" as="dd" className="text-content-primary min-w-0 break-all">
        {label}
        {level !== '' && (
          <Badge className="ml-2" tone="neutral">
            {t(`level.${level}`)}
          </Badge>
        )}
        {flag && (
          <Badge className="ml-2" tone="warning">
            {t(`recommendationSets.flag.${flag}`)}
          </Badge>
        )}
      </Typography>
    </div>
  )
}

interface SlotReading {
  label: string
  level: AdminCatalogEntry['level']
  flag: SlotFlag | undefined
}

type SlotFlagReader = (stage: StageName, ref: ModelRef) => SlotReading

const EMPTY_SLOT: SlotReading = { label: '', level: '', flag: undefined }

/** Reads each slot against the catalog as it is now (MODEL-70's read-time flags): the stage's
 *  own purpose listing says whether the model is still registered there and at which grade.
 *  Nothing is flagged while a listing is loading or failed — a missing answer is not evidence. */
function useSlotFlags(): SlotFlagReader {
  const { models } = useModels()
  const catalogs: Record<StageName, ReturnType<typeof useAdminCatalog>> = {
    observe: useAdminCatalog(STAGE_PURPOSE.observe),
    analyze: useAdminCatalog(STAGE_PURPOSE.analyze),
    write: useAdminCatalog(STAGE_PURPOSE.write),
  }
  return (stage, ref) => {
    const listing = catalogs[stage]
    const entry = listing.catalog.entries.find((candidate) => candidate.modelId === ref.modelId)
    const label =
      models.find((model) => sameRef(model.ref, ref))?.label ?? entry?.label ?? ref.modelId
    if (listing.isPending || listing.isError) return { label, level: '', flag: undefined }
    if (!entry || !entry.purposes.includes(STAGE_PURPOSE[stage])) {
      return { label, level: '', flag: 'unregistered' }
    }
    return { label, level: entry.level, flag: entry.level === '' ? 'unclassified' : undefined }
  }
}
