import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { appFailureFromConnect } from '@/shared/api'
import {
  type GenerationOptionsSet,
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
  POST_TARGET_LENGTH_DEFAULT,
  POST_TARGET_LENGTH_MAX,
  POST_TARGET_LENGTH_MIN,
  useGenerationOptions,
} from '@/entities/post'
import {
  AppFailureMessage,
  Button,
  Checkbox,
  FieldLabel,
  FieldMessage,
  Notice,
  TextField,
  typographyStyles,
} from '@/shared/ui'
import { formatNumber } from '@/shared/lib'
import {
  changedFrom,
  draftFromSet,
  lengthValid,
  setFromDraft,
  tagsValid,
  type RunOptionsDraft,
} from '../model/run-options-form'

type FormMember = Pick<GenerationOptionsSet, 'useMemory' | 'qualityRules' | 'field'>

/** What the brief widget's controls read and change inside the form: the three run options that
 *  are choices rather than numbers. `disabled` holds them on a published post and while the save
 *  is out; `jobRunning` is for the one control a running job holds too (the ticks). */
export interface RunOptionsForm {
  values: FormMember
  change: (patch: Partial<FormMember>) => void
  disabled: boolean
  jobRunning: boolean
}

/** The writing brief's run options as ONE form saved by its 저장 (POST-89): 목표 분량 and 태그 개수
 *  here, then whatever the brief renders into `children` (발행 글 점검, 분야, 기억 사용). A tick, a
 *  chip or a typed number sends nothing; 저장 sends the whole set in one request and closes the
 *  brief once it lands, and closing without it discards the change. The form is seeded once, from
 *  the saved set, so a refetch while the brief is open never throws away what the user typed.
 *
 *  It renders as a form BODY, with no surface of its own: the brief widget owns the overlay it
 *  sits in (`widgets/generation-brief`), and a popover inside a popover is not a shape. */
export function GenerationOptions({
  slug,
  saved,
  locked,
  jobRunning,
  onSaved,
  onClose,
  children,
}: {
  slug: string
  saved: GenerationOptionsSet
  /** A published post (POST-86): the whole form and 저장 are held. */
  locked: boolean
  /** A job is running: the numbers and the ticks are held, 분야 and 기억 사용 stay usable. */
  jobRunning: boolean
  onSaved: (set: GenerationOptionsSet) => void
  /** Dismisses the surface this sits in, on 취소 and on a landed save. */
  onClose: () => void
  children?: (form: RunOptionsForm) => ReactNode
}) {
  const { t } = useTranslation(['posts', 'common'])
  const save = useGenerationOptions()
  const [draft, setDraft] = useState<RunOptionsDraft>(() => draftFromSet(saved))
  const pending = save.isPending
  const update = (patch: Partial<RunOptionsDraft>) =>
    setDraft((current) => ({ ...current, ...patch }))
  const lengthOk = lengthValid(draft)
  const tagsOk = tagsValid(draft)
  const canSave = !locked && !pending && changedFrom(draft, saved)
  const numbersDisabled = locked || jobRunning || pending
  const form: RunOptionsForm = {
    values: { useMemory: draft.useMemory, qualityRules: draft.qualityRules, field: draft.field },
    change: update,
    disabled: locked || pending,
    jobRunning,
  }

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        // The same condition as the button's, so Enter cannot send a second request.
        if (!canSave) return
        const next = setFromDraft(draft)
        void save
          .save(slug, next)
          .then(() => {
            onSaved(next)
            onClose()
          })
          .catch(() => undefined)
      }}
    >
      <label
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 items-center gap-3',
        })}
      >
        <Checkbox
          checked={draft.lengthOn}
          disabled={numbersDisabled}
          // Ticking the box reveals the field with a usable number ALREADY in it. Revealing an
          // empty one puts a range error under a control nobody has touched yet, and asks the
          // user to invent a character count before they have any reason to have one. A value
          // they typed earlier outranks the default, so unticking and reticking never loses it.
          onChange={(event) => {
            const on = event.target.checked
            update({
              lengthOn: on,
              length: on && !draft.length ? String(POST_TARGET_LENGTH_DEFAULT) : draft.length,
            })
          }}
        />
        {t('generation.options.useTarget', { ns: 'posts' })}
      </label>
      {draft.lengthOn && (
        <div className="mt-3">
          <FieldLabel htmlFor={`generation-target-${slug}`}>
            {t('generation.options.target', { ns: 'posts' })}
          </FieldLabel>
          <TextField
            id={`generation-target-${slug}`}
            type="number"
            min={POST_TARGET_LENGTH_MIN}
            max={POST_TARGET_LENGTH_MAX}
            value={draft.length}
            disabled={numbersDisabled}
            onChange={(event) => update({ length: event.target.value })}
            aria-invalid={!lengthOk || undefined}
            className="mt-1"
          />
          {!lengthOk && (
            <FieldMessage className="mt-1">
              {t('generation.options.range', {
                ns: 'posts',
                min: formatNumber(POST_TARGET_LENGTH_MIN),
                max: formatNumber(POST_TARGET_LENGTH_MAX),
              })}
            </FieldMessage>
          )}
        </div>
      )}
      {/* Always visible, no enabling tick: a tag list has no "natural" count to fall back to, so
          the field arrives holding the number the next run will use (POST-63). */}
      <div className="mt-3">
        <FieldLabel htmlFor={`generation-tags-${slug}`}>
          {t('generation.options.tagCount', { ns: 'posts' })}
        </FieldLabel>
        <TextField
          id={`generation-tags-${slug}`}
          type="number"
          min={POST_TAG_COUNT_MIN}
          max={POST_TAG_COUNT_MAX}
          value={draft.tags}
          disabled={numbersDisabled}
          onChange={(event) => update({ tags: event.target.value })}
          aria-invalid={!tagsOk || undefined}
          className="mt-1"
        />
        {!tagsOk && (
          <FieldMessage className="mt-1">
            {t('generation.options.tagCountRange', {
              ns: 'posts',
              min: POST_TAG_COUNT_MIN,
              max: POST_TAG_COUNT_MAX,
            })}
          </FieldMessage>
        )}
      </div>
      {children && <div className="mt-4 grid grid-cols-1 gap-4 *:min-w-0">{children(form)}</div>}
      {save.error && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={appFailureFromConnect(save.error)} />
        </Notice>
      )}
      <div className="mt-4 flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose}>
          {t('action.cancel', { ns: 'common' })}
        </Button>
        <Button type="submit" variant="secondary" disabled={!canSave} pending={pending}>
          {t('action.save', { ns: 'common' })}
        </Button>
      </div>
    </form>
  )
}
