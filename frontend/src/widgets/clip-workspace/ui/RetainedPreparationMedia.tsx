import { useEffect, useRef, type ReactNode, type SyntheticEvent } from 'react'

/** Retain controls, but stop current and late-starting media while their screen is hidden. */
export function RetainedPreparationMedia({
  active,
  children,
}: {
  active: boolean
  children: ReactNode
}) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (active) return
    root.current?.querySelectorAll<HTMLMediaElement>('video, audio').forEach((media) => {
      if (!media.paused) media.pause()
    })
  }, [active])
  function guardPlayback(event: SyntheticEvent<HTMLDivElement>) {
    if (active || !(event.target instanceof HTMLMediaElement)) return
    // Downloads and play promises can finish after navigation. Capture before a sample's
    // playing handler can acknowledge hidden playback or expose an unreachable Stop control.
    event.stopPropagation()
    event.target.pause()
  }
  return (
    <div ref={root} onPlayCapture={guardPlayback} onPlayingCapture={guardPlayback}>
      {children}
    </div>
  )
}
