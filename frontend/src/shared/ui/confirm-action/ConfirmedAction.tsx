import { useState, type ReactNode } from 'react'
import { Dialog } from '../dialog/Dialog'

interface ConfirmedActionProps {
  /** The control that asks for confirmation; `open` raises the dialog. */
  trigger: (open: () => void) => ReactNode
  /** Whether the last run failed, so `failure` shows beside the trigger. */
  failed: boolean
  failure: ReactNode
  title: string
  confirmLabel: string
  pending: boolean
  /** The explanation the dialog shows. */
  children: ReactNode
  /** The confirmed action; a rejection means it failed. */
  run: () => Promise<unknown>
  /** After a successful run, once the dialog is closed. */
  onDone?: () => void | Promise<void>
}

/** The confirm-then-run shape of a destructive control: trigger, its failure message, then the
 *  dialog. Confirming runs the action and closes the dialog either way, so a failure message is
 *  never left behind the scrim; only a success calls `onDone`. */
export function ConfirmedAction({
  trigger,
  failed,
  failure,
  title,
  confirmLabel,
  pending,
  children,
  run,
  onDone,
}: ConfirmedActionProps) {
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await run()
    } catch {
      // The failure renders beside the trigger.
      setConfirming(false)
      return
    }
    setConfirming(false)
    await onDone?.()
  }

  return (
    <>
      {trigger(() => setConfirming(true))}
      {failed && failure}
      <Dialog
        open={confirming}
        title={title}
        confirmLabel={confirmLabel}
        pending={pending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {children}
      </Dialog>
    </>
  )
}
