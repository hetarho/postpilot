import { useTranslation } from 'react-i18next'
import {
  guidelineErrorMessage,
  useSetDefaultGuidelineEnabled,
  type DefaultGuideline,
  type GuidelineKind,
} from '@/entities/guideline'
import { Button } from '@/shared/ui'

/** `적용 안함` on an open 기본 지침 (GUIDE-43): takes it out of use at once, with no dialog —
 *  putting it back is one press in the 기본 지침 sheet. The write is optimistic, so the row leaves
 *  the list before the answer; a refusal puts it back, and because the row this button sat in is
 *  gone by then, the message goes to `onRefused` for the list to say above itself. */
export function StopDefaultGuidelineButton({
  ownerId,
  kind,
  guideline,
  onRefused,
}: {
  ownerId: string
  kind: GuidelineKind
  guideline: Pick<DefaultGuideline, 'key' | 'name'>
  /** Called with the catalogue's message on a refusal, and with `''` when a new attempt starts. */
  onRefused: (message: string) => void
}) {
  const { t } = useTranslation('guidelines')
  const toggle = useSetDefaultGuidelineEnabled(ownerId, kind)
  return (
    <Button
      variant="secondary"
      aria-label={t('defaults.stopNamed', { name: guideline.name })}
      disabled={toggle.isPending}
      onClick={() => {
        onRefused('')
        toggle
          .setEnabled(guideline.key, false)
          .catch((cause: unknown) => onRefused(guidelineErrorMessage(cause)))
      }}
    >
      {t('defaults.stop')}
    </Button>
  )
}
