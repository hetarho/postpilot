import { useTranslation } from 'react-i18next'
import { Notice, Typography } from '@/shared/ui'
import type { SaveState } from '../model/draft-queue'

/** Nothing at all while the editor is untouched: a "저장됨" on a post nobody has edited
 *  yet would claim a save that never happened. */
export function SaveStatus({ state }: { state: SaveState }) {
  const { t } = useTranslation('common')
  const label = {
    idle: '',
    dirty: t('state.savePending'),
    saving: t('action.saving'),
    saved: t('state.saved'),
    error: t('state.saveRetrying'),
  }[state]
  return (
    // The live region is this wrapper and only its CONTENTS change. A region that is inserted
    // already holding its message is not reliably announced, so swapping the element itself for
    // the failure state would silence exactly the state that matters most.
    //
    // Polite, not assertive: this changes about once a second while someone types, and a
    // screen reader interrupting every keystroke would make the editor unusable.
    <div role="status" aria-live="polite" data-state={state}>
      {state === 'error' ? (
        // A failed autosave is the only thing this screen has to say instead of a save button
        // (PRD F-2), so it takes the §2.6 notice contract rather than 12px of bare red text.
        <Notice tone="danger">{label}</Notice>
      ) : (
        <Typography
          variant="body"
          as="p"
          className={state === 'saved' ? 'text-notice-success-fg' : 'text-content-tertiary'}
        >
          {label}
        </Typography>
      )}
    </div>
  )
}
