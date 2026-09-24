import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useGenerationOptions } from '@/entities/post'
import { Checkbox, FieldMessage, typographyStyles } from '@/shared/ui'

/** ①'s memory opt-in (POST-71, MEM-18). It sits at the foot of the panel rather than in the
 *  writing brief because it silently changes what a run may write — the same reason 말투 and
 *  템플릿 stay readable without opening anything (POST-51).
 *
 *  It autosaves on the toggle: the flag is an option of the next RUN, so saving it moves no
 *  status, revision, baseline or learning eligibility, and there is nothing to confirm. */
export function UseMemoriesField({
  slug,
  useMemory,
  targetLength,
  disabled,
  className,
}: {
  slug: string
  useMemory: boolean
  /** The post's 목표 글자 수, resent with the flag: this save reads an absent length as natural
   *  length, so leaving it out would clear the number the brief saved. */
  targetLength?: number
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
      await options.saveUseMemory(slug, next, targetLength)
    } catch {
      setOptimistic(null)
    }
  }

  return (
    <div className={className}>
      {/* The primitive owns the 44px target; the label wraps it so the words are part of it. No
          help line follows: the box shares one row with 분야 (owner decision 2026-09-25). */}
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
      {options.isError && (
        <FieldMessage className="mt-1">{t('useMemories.saveFailed')}</FieldMessage>
      )}
    </div>
  )
}
