import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import {
  LEVELS,
  levelOf,
  modelChoiceIssue,
  orderModelsForStage,
  postCreditLabel,
  refKey,
  stagePostFigure,
  useModels,
  useSaveSelection,
  useSelections,
  useStageSelection,
  type CatalogModel,
  type StageName,
} from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Badge,
  Button,
  Notice,
  SegmentedControl,
  Sheet,
  Typography,
  cardStyles,
  typographyStyles,
} from '@/shared/ui'

/** `grade` is the order the picker opens on and no switch names it: 무료 first, then 가성비 up to
 *  최고, unclassified last (MODEL-96 →MODEL-57). The three named lenses apply only once the owner
 *  picks one. */
type Lens = 'grade' | 'quality' | 'balanced' | 'price'
const NAMED_LENSES = ['quality', 'balanced', 'price'] as const

function levelRank(model: CatalogModel, stage: StageName): number {
  const level = levelOf(model, stage)
  return level === undefined ? -1 : LEVELS.indexOf(level)
}
function credits(model: CatalogModel, stage: StageName): number {
  if (model.access?.[stage]?.grade === 'free') return 0
  return stagePostFigure(model, stage)?.credits ?? Number.POSITIVE_INFINITY
}

/** The stage's models in the order the lens asks for. The opening order is the selectors' shared
 *  grade order. 품질 먼저: highest grade first. 가격 먼저: fewest credits per post first (free
 *  first). 균형: the grade nearest 밸런스 first. Inside one rank the catalog's own order survives
 *  (a stable sort). */
function orderByLens(
  models: readonly CatalogModel[],
  stage: StageName,
  lens: Lens,
): CatalogModel[] {
  if (lens === 'grade') return orderModelsForStage(models, stage)
  const middle = LEVELS.indexOf('balanced')
  const key = (model: CatalogModel) =>
    lens === 'quality'
      ? -levelRank(model, stage)
      : lens === 'price'
        ? credits(model, stage)
        : levelRank(model, stage) < 0
          ? LEVELS.length
          : Math.abs(levelRank(model, stage) - middle)
  return [...models].sort((a, b) => key(a) - key(b))
}

/** A failed catalog or selections read is its own answer (MODEL-24): neither one may read as
 *  "nothing chosen", and its retry asks both again. */
function useCatalogRead() {
  const models = useModels()
  const selections = useSelections()
  return {
    failed: models.isError || selections.isError,
    retry: () => {
      models.refetch()
      selections.refetch()
    },
  }
}

/** Guide: one stage's AI as a row — the plain name of the stage, the model in use with its grade
 *  and what one post costs — whose 바꾸기 opens the picker as a bottom sheet. The picker is a
 *  list of big radio cards, opening free first and resorted by what the owner says matters first
 *  (품질 · 균형 · 가격), and a tap saves the choice at once: the same save the old dropdown made. */
export function StageModelRow({ stage }: { stage: StageName }) {
  const { t } = useTranslation(['models', 'common'])
  const active = useStageSelection(stage)
  const read = useCatalogRead()
  const [open, setOpen] = useState(false)
  const reference = active.selected ?? active.unavailable?.ref
  const current = reference
    ? active.models.find((model) => refKey(model.ref) === refKey(reference))
    : undefined
  const name = current?.label ?? (reference ? refKey(reference) : t('picker.none'))
  const figure = current ? stagePostFigure(current, stage) : undefined
  const level = current ? levelOf(current, stage) : undefined
  const headingId = useId()
  return (
    <div className="flex min-h-16 items-center gap-3 px-4 py-3">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className={typographyStyles({ variant: 'label' })}>{t(`picker.stage.${stage}`)}</span>
        {read.failed ? (
          <span className={typographyStyles({ variant: 'body', className: 'text-field-error' })}>
            {t('picker.rowFailed')}
          </span>
        ) : (
          <span className={typographyStyles({ variant: 'fieldTitle', className: 'truncate' })}>
            {active.isPending ? '—' : name}
          </span>
        )}
        <span className="flex flex-wrap items-center gap-2">
          {level && <Badge tone="accent">{t(`level.${level}`)}</Badge>}
          {figure && (
            <span className={typographyStyles({ variant: 'meta' })}>{postCreditLabel(figure)}</span>
          )}
          {active.unavailable && (
            <span className={typographyStyles({ variant: 'meta', className: 'text-field-error' })}>
              {active.unavailable.reason}
            </span>
          )}
        </span>
      </div>
      {read.failed ? (
        <Button
          variant="secondary"
          onClick={read.retry}
          aria-label={`${t('action.retry', { ns: 'common' })} · ${t(`picker.stage.${stage}`)}`}
        >
          {t('action.retry', { ns: 'common' })}
        </Button>
      ) : (
        <Button
          variant="secondary"
          onClick={() => setOpen(true)}
          aria-label={`${t('picker.change')} · ${t(`picker.stage.${stage}`)}`}
          disabled={active.isPending}
        >
          {t('picker.change')}
        </Button>
      )}
      <Sheet
        open={open}
        labelledBy={headingId}
        onClose={() => setOpen(false)}
        header={
          <Typography variant="title" as="h2" id={headingId} className="px-5 pt-5 sm:px-6">
            {t('picker.title', { stage: t(`picker.stage.${stage}`) })}
          </Typography>
        }
        bodyClassName="px-5 pb-4 sm:px-6"
        footer={
          <div className="px-5 pt-2 pb-5 sm:px-6">
            <Button variant="cta" size="hero" className="w-full" onClick={() => setOpen(false)}>
              {t('picker.done')}
            </Button>
          </div>
        }
      >
        <ModelPicker stage={stage} />
      </Sheet>
    </div>
  )
}

/** The picker itself: a lens switch, none of whose lenses is on until the owner picks one, and
 *  the stage's models as radio cards. */
export function ModelPicker({ stage }: { stage: StageName }) {
  const { t } = useTranslation(['models', 'common'])
  const active = useStageSelection(stage)
  const read = useCatalogRead()
  const save = useSaveSelection()
  const [lens, setLens] = useState<Lens>('grade')
  const selectedKey = active.selected ? refKey(active.selected) : ''
  const pendingKey =
    save.isPending && save.variables?.ref
      ? refKey({
          providerId: save.variables.ref.providerId ?? '',
          modelId: save.variables.ref.modelId ?? '',
        })
      : ''
  const models = orderByLens(active.models, stage, lens)
  if (read.failed)
    return (
      <Notice tone="danger" role="alert">
        {t('picker.listFailed')}
        <Button variant="ghost" className="mt-3" onClick={read.retry}>
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Notice>
    )
  return (
    <div className="flex flex-col gap-4">
      <SegmentedControl
        value={lens}
        onChange={(value) => setLens(value as Lens)}
        options={NAMED_LENSES.map((value) => ({
          value,
          label: t(`picker.lens.${value}`),
        }))}
        ariaLabel={t('picker.lensAria')}
      />
      <div
        role="radiogroup"
        aria-label={t('picker.choices', { stage: t(`picker.stage.${stage}`) })}
        className="flex flex-col gap-3"
      >
        {models.map((model) => {
          const key = refKey(model.ref)
          const issue = modelChoiceIssue(model, stage)
          const level = levelOf(model, stage)
          const figure = stagePostFigure(model, stage)
          const free = model.access?.[stage]?.grade === 'free'
          const checked = (pendingKey || selectedKey) === key
          return (
            <button
              key={key}
              type="button"
              role="radio"
              aria-checked={checked}
              disabled={!!issue || save.isPending}
              onClick={() => void save.save(stage, model.ref)}
              className={cardStyles({
                padding: 'sm',
                interactive: !issue,
                className: clsx(
                  'flex w-full items-center gap-3 text-left disabled:cursor-not-allowed',
                  checked && 'ring-interaction-focus ring-2',
                  issue && 'opacity-60',
                ),
              })}
            >
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="flex flex-wrap items-center gap-2">
                  {level && <Badge tone="accent">{t(`level.${level}`)}</Badge>}
                  {checked && <Badge tone="success">{t('picker.inUse')}</Badge>}
                </span>
                <span
                  className={typographyStyles({ variant: 'fieldTitle', className: 'break-words' })}
                >
                  {model.label}
                </span>
                <span className={typographyStyles({ variant: 'meta' })}>
                  {issue || (free ? t('picker.free') : figure ? postCreditLabel(figure) : '')}
                </span>
              </span>
              <span
                aria-hidden="true"
                className={clsx(
                  'flex size-7 shrink-0 items-center justify-center rounded-full',
                  checked ? 'bg-button-cta-bg text-button-cta-fg' : 'bg-progress-step-todo',
                )}
              >
                {checked && <Check className="size-4" />}
              </span>
            </button>
          )
        })}
      </div>
      {save.failure && (
        <Typography variant="body" as="div" role="alert" className="text-field-error">
          <AppFailureMessage failure={save.failure} />
        </Typography>
      )}
    </div>
  )
}
