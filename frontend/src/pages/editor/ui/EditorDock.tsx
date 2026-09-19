import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ActionBar } from '@/shared/ui'

/** The editor's docked bar. It holds the one thing a phone could not otherwise reach — the step's
 *  committing action, which sat in normal flow roughly 1,000px down a 4,000px page (§4.3) — the
 *  brief and the assignments that decide what that action does, and the reason the action is
 *  refused. Nothing that is merely TRUE: the save state, the job's progress and the post's status
 *  all moved to the page-top status region (`EditorStatus`, change 15).
 *
 *  It is mounted as the LAST child of `main` on template: a `sticky bottom-*` box is only pinned
 *  while its containing block still extends below it, so anywhere earlier in the flow it would
 *  scroll away with the section it sat in. */
export function EditorDock({
  header,
  children,
}: {
  /** The bar's first row: 글 생성 puts 말투 and the writing brief's glyph here, above whatever the
   *  step's own actions are. */
  header?: ReactNode
  children?: ReactNode
}) {
  const { t } = useTranslation('posts')
  // A bar holding neither a control nor a refusal is not chrome (§0). Now that no status line
  // lives here, that is the whole test.
  if (!children && !header) return null
  return (
    <>
      {/* `mt-auto` alone can resolve to zero when the page is taller than the viewport. Keep a
          real 24px spacer as well, so the step panel never touches the dock card. */}
      <div aria-hidden className="mt-auto h-6 shrink-0" />
      <ActionBar ariaLabel={t('editor.actionAria')} className="mt-0">
        <div className="flex flex-col gap-2">
          {header}
          {children}
        </div>
      </ActionBar>
    </>
  )
}
