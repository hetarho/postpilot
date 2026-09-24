import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  GuidelineFieldPicker,
  useUpdateGuidelinePresetCall,
  type GuidelinePreset,
} from '@/entities/guideline'
import { FieldLabel, FieldMessage, Notice, Switch, Typography } from '@/shared/ui'

/** The product's 상위 노출 단어 사용, always the first thing on /guidelines (GUIDE-29, GUIDE-38).
 *
 *  Its text is the server's and has no edit control (GUIDE-32); what the owner decides is whether
 *  to adopt it — its own switch — and for which 분야, a set that IS its `fields` scope. Picking
 *  while it is off is allowed: a set chosen first is what switching on then applies to.
 *
 *  The row shows the state the server answered, never a guess: each save writes its answer into
 *  the list entry, and the controls hold still while one is out, so no pick is computed from a
 *  set that is about to change. A refusal is the failure catalogue's words (GUIDE-24). */
export function GuidelinePresetRow({
  ownerId,
  preset,
}: {
  ownerId: string
  preset: GuidelinePreset
}) {
  const { t } = useTranslation('guidelines')
  const nameId = useId()
  const switchId = useId()
  const update = useUpdateGuidelinePresetCall(ownerId)
  // The refusal is shown from the mutation's own error, so the promise is only settled here.
  const save = (patch: () => Promise<unknown>) => void patch().catch(() => undefined)

  return (
    <section aria-labelledby={nameId} className="mt-8">
      <div className="flex min-h-11 items-center gap-3">
        {/* The visible name labels the switch itself, so the switch is announced by it. */}
        <FieldLabel id={nameId} htmlFor={switchId} className="min-w-0 flex-1">
          {t('preset.name')}
        </FieldLabel>
        <Switch
          id={switchId}
          checked={preset.enabled}
          disabled={update.isPending}
          onChange={(event) => save(() => update.setEnabled(event.target.checked))}
        />
      </div>
      <Typography variant="body" as="p" className="mt-2 whitespace-pre-wrap">
        {preset.text}
      </Typography>
      <Typography variant="body" as="p" className="text-content-secondary mt-2">
        {t('preset.about')}
      </Typography>
      <Typography variant="body" as="p" className="text-content-secondary mt-1">
        {t('preset.yields')}
      </Typography>
      <GuidelineFieldPicker
        value={preset.fields}
        onChange={(fields) => save(() => update.setFields(fields))}
        legend={t('preset.fields')}
        legendVisible
        disabled={update.isPending}
        className="mt-3"
      />
      {/* On with no 분야 reaches no post, which is worth saying where the fix is (GUIDE-38). */}
      {preset.enabled && preset.fields.length === 0 && (
        <Notice tone="warning" role="status" className="mt-3">
          {t('preset.needsField')}
        </Notice>
      )}
      {update.isError && update.errorMessage && (
        <FieldMessage className="mt-2">{update.errorMessage}</FieldMessage>
      )}
    </section>
  )
}
