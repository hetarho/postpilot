import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useGenerationOptions } from '@/entities/post'
import { Checkbox, FieldMessage, Typography, typographyStyles } from '@/shared/ui'

/** ①'s memory opt-in (POST-71, MEM-18). It sits at the foot of the panel rather than in the
 *  writing brief because it silently changes what a run may write — the same reason 말투 and
 *  템플릿 stay readable without opening anything (POST-51).
 *
 *  It autosaves on the toggle: the flag is an option of the next RUN, so saving it moves no
 *  status, revision, baseline or learning eligibility, and there is nothing to confirm. */
export function UseMemoriesField({
  slug,
  useMemory,
  disabled,
  className,
}: {
  slug: string
  useMemory: boolean
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('posts')
  const options = useGenerationOptions()
  // The press is answered immediately and the POST is the truth. The optimistic value holds only
  // until the post catches up — the expression below falls back to the prop the moment the two
  // agree, so the refetch that follows a save needs no effect to clear it, and a box that has
  // just been ticked cannot flicker back while that read is in flight. A refused save clears it
  // explicitly: nothing was saved, so the box must go back to what the post says.
  const [optimistic, setOptimistic] = useState<boolean | null>(null)
  const checked = optimistic === null ? useMemory : optimistic

  const toggle = async (next: boolean) => {
    setOptimistic(next)
    try {
      await options.saveUseMemory(slug, next)
    } catch {
      setOptimistic(null)
    }
  }

  return (
    <div className={className}>
      {/* The primitive owns the 44px target; the label wraps it so the words are part of it. */}
      <label
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 items-center gap-3',
        })}
      >
        <Checkbox
          checked={checked}
          disabled={disabled || options.isPending}
          onChange={(event) => void toggle(event.target.checked)}
        />
        {t('useMemories.label')}
      </label>
      <Typography variant="meta" as="p" className="text-content-secondary mt-1">
        {t('useMemories.help')}
      </Typography>
      {options.isError && (
        <FieldMessage className="mt-1">{t('useMemories.saveFailed')}</FieldMessage>
      )}
    </div>
  )
}
