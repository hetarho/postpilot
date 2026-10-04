import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Typography } from '@/shared/ui'
import { useSpokenActions } from '../api/hooks'
import { SPOKEN_MAX_AUDIO_BYTES } from '../config/limits'
interface Props {
  ownerId: string
  assetId: string
  name: string
  durationMs: number
  disabled?: boolean
  onPlayed?: (playbackId: string) => Promise<void>
  onFailure?: (error: unknown) => void
}
export function SpokenSamplePlayer(props: Props) {
  return <SamplePlayer key={`${props.ownerId}:${props.assetId}`} {...props} />
}
function SamplePlayer({
  ownerId,
  assetId,
  name,
  durationMs,
  disabled,
  onPlayed,
  onFailure,
}: Props) {
  const { t } = useTranslation('spokenVoice'),
    actions = useSpokenActions(ownerId)
  const audio = useRef<HTMLAudioElement>(null),
    abort = useRef(new AbortController()),
    ticket = useRef<{ id: string; url: string; expires: number; acknowledged: boolean } | null>(
      null,
    )
  const [loading, setLoading] = useState(false),
    [playing, setPlaying] = useState(false)
  useEffect(() => {
    abort.current = new AbortController()
    const element = audio.current,
      controller = abort.current
    return () => {
      controller.abort()
      element?.pause()
      if (ticket.current) URL.revokeObjectURL(ticket.current.url)
      ticket.current = null
    }
  }, [])
  async function play() {
    try {
      if (!ticket.current || ticket.current.expires <= Date.now()) {
        setLoading(true)
        const access = await actions.sampleAccess(assetId)
        if (abort.current.signal.aborted) return
        const response = await fetch(access.url, {
          credentials: 'include',
          cache: 'no-store',
          signal: abort.current.signal,
        })
        if (!response.ok || !response.headers.get('content-type')?.startsWith('audio/mpeg'))
          throw new Error('Private sample unavailable')
        const blob = await response.blob()
        if (blob.size === 0 || blob.size > SPOKEN_MAX_AUDIO_BYTES)
          throw new Error('Invalid private sample')
        if (abort.current.signal.aborted) return
        if (ticket.current) URL.revokeObjectURL(ticket.current.url)
        ticket.current = {
          id: access.playbackId,
          url: URL.createObjectURL(blob),
          expires: Date.parse(access.expiresAt),
          acknowledged: false,
        }
        if (audio.current) audio.current.src = ticket.current.url
      }
      if (abort.current.signal.aborted) return
      await audio.current?.play()
    } catch (error) {
      if (!abort.current.signal.aborted) onFailure?.(error)
    } finally {
      if (!abort.current.signal.aborted) setLoading(false)
    }
  }
  function played() {
    const current = ticket.current
    if (!current || abort.current.signal.aborted) return
    setPlaying(true)
    if (!current.acknowledged && onPlayed) {
      current.acknowledged = true
      void onPlayed(current.id).catch((error) => {
        current.acknowledged = false
        if (!abort.current.signal.aborted) onFailure?.(error)
      })
    }
  }
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-3">
      <audio
        ref={audio}
        preload="none"
        onPlaying={played}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
        aria-hidden="true"
      />
      <Button
        variant="secondary"
        disabled={disabled}
        pending={loading}
        onClick={() => {
          if (playing) audio.current?.pause()
          else void play()
        }}
      >
        {playing ? t('stop') : t('play', { name })}
      </Button>
      <Typography variant="meta">
        {t('duration', { seconds: (durationMs / 1000).toFixed(1) })}
      </Typography>
    </div>
  )
}
