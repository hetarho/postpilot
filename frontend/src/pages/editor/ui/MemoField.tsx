import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { FieldLabel, Textarea } from '@/shared/ui'

/** The memo is the post's own words, and it is what 글 생성 works from — so it is rendered by
 *  that step while its value and its autosave stay in `DraftEditor`. Unmounting the field on
 *  another step cannot strand a queued save: the queue is keyed by slug and lives above it. */
export function MemoField({
  value,
  onChange,
  fieldRef,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-memo" className="sr-only">
        {t('editor.memo')}
      </FieldLabel>
      {/* `autoGrow` with a small `rows`: at 16 rows the memo was a 364px box scrolling inside
          itself, which swallowed every vertical swipe that landed on it and left the 16px gutters
          as the only place to scroll the page (§4.4). The well appearance owns the §3.1 input
          size, so the focused field remains at least 16px on a phone. */}
      <Textarea
        id="post-memo"
        ref={fieldRef}
        appearance="well"
        rows={6}
        autoGrow
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t('editor.memoPlaceholder')}
        enterKeyHint="enter"
        className="mt-5"
      />
    </>
  )
}
