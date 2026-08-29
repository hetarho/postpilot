import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** An inline block of feedback on the page — the §2.6 notice contract, made a primitive because
 *  eight slices were inlining the same `bg-notice-*-bg text-notice-*-fg rounded-md px-3 py-2`
 *  string (§1.1).
 *
 *  Colour never travels alone: a notice always renders its explanatory text, and the tone only
 *  reinforces what the words already say. `px-4 py-3` is the §4.2 ratio for a text block rather
 *  than a control. */
export type NoticeTone = 'danger' | 'success' | 'warning' | 'info'

const TONE_STYLES: Record<NoticeTone, string> = {
  danger: 'bg-notice-danger-bg text-notice-danger-fg',
  success: 'bg-notice-success-bg text-notice-success-fg',
  warning: 'bg-notice-warning-bg text-notice-warning-fg',
  info: 'bg-notice-info-bg text-notice-info-fg',
}

export function Notice({
  tone,
  children,
  className,
  role,
}: {
  tone: NoticeTone
  children: ReactNode
  /** `alert` for something that went wrong and interrupts, `status` for progress and confirmation.
   *  Always pass one when the notice appears in response to an action (§9). */
  role?: 'alert' | 'status'
  className?: string
}) {
  return (
    <div
      role={role}
      className={twMerge(
        'flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md px-4 py-3 text-sm break-words',
        TONE_STYLES[tone],
        className,
      )}
    >
      {children}
    </div>
  )
}
