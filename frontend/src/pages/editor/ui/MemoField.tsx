import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { FieldLabel, Textarea } from '@/shared/ui'
import { MEMO_WORKSPACE_RESERVED_BOTTOM_PX } from '../config/workspace'

/** The memo is the post's own words, and it is what 글 생성 works from — so it is rendered by
 *  that step while its value and its autosave stay in `DraftEditor`. Unmounting the field on
 *  another step cannot strand a queued save: the queue is keyed by slug and lives above it. */
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
        rows={6}
        autoGrow
        viewportAllocation={{ reservedBottom: MEMO_WORKSPACE_RESERVED_BOTTOM_PX }}
        value={value}
        readOnly={readOnly}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t('editor.memoPlaceholder')}
        enterKeyHint="enter"
        className="mt-5"
      />
    </>
  )
}
