import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { FieldLabel, Textarea } from '@/shared/ui'

/** Optional closing material after the template answers; its autosave stays in DraftEditor. */
export function MemoField({
  value,
  onChange,
  fieldRef,
  readOnly = false,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
  /** A published post's memo: readable and selectable, never edited (POST-86). */
  readOnly?: boolean
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-memo" className="sr-only">
        {t('editor.memo')}
      </FieldLabel>
      {/* `autoGrow` with a small `rows`: at 16 rows the memo was a 364px box scrolling inside
          itself, which swallowed every vertical swipe that landed on it and left the 16px gutters
          as the only place to scroll the page (THEME-25). The well appearance owns the THEME-20 input
          size, so the focused field remains at least 16px on a phone. */}
      <Textarea
        id="post-memo"
        ref={fieldRef}
        appearance="well"
        // Optional closing notes follow the photos and template answers.
        rows={3}
        autoGrow
        value={value}
        readOnly={readOnly}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t('guide.memoPlaceholder')}
        enterKeyHint="enter"
        className="bg-card-bg rounded-xl p-5 shadow-md"
      />
    </>
  )
}
