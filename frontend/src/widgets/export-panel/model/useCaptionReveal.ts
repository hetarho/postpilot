import { useEffect, useRef } from 'react'

/** The Naver body's fallback reveal, one caption down (EXPORT-24): the caption's field does not
 *  exist until its copy is refused, so the selection has to happen once it is mounted. And when
 *  the fallback dissolves under the user — a content change — the focused field unmounts, which
 *  would drop the keyboard onto <body>; it is handed back to the control that was pressed.
 *
 *  The caller attaches the two refs: `fieldRef` to the revealed field, `controlRef` to the
 *  caption's own copy control. */
export function useCaptionReveal(fellBack: boolean) {
  const controlRef = useRef<HTMLButtonElement>(null)
  const fieldRef = useRef<HTMLInputElement>(null)
  const wasRevealed = useRef(false)
  useEffect(() => {
    if (fellBack) {
      wasRevealed.current = true
      fieldRef.current?.focus()
      fieldRef.current?.select()
      return
    }
    if (wasRevealed.current) {
      wasRevealed.current = false
      if (document.activeElement === document.body) controlRef.current?.focus()
    }
  }, [fellBack])
  return { controlRef, fieldRef }
}
