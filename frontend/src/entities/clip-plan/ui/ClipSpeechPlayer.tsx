import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Typography } from '@/shared/ui'
import type { ClipSpeechRef } from '../model/spoken'
import { useClipSpeechCalls } from '../api/speech'
export function ClipSpeechPlayer(props: {
  ownerId: string
  projectId: string
  speech: ClipSpeechRef
  previous: boolean
  disabled?: boolean
}) {
  return (
    <SpeechPlayer key={`${props.ownerId}:${props.projectId}:${props.speech.assetId}`} {...props} />
  )
}
function SpeechPlayer({
  projectId,
  speech,
  previous,
  disabled,
}: {
  projectId: string
  speech: ClipSpeechRef
  previous: boolean
  disabled?: boolean
  ownerId: string
}) {
  const { t } = useTranslation('clips'),
    calls = useClipSpeechCalls(),
    audio = useRef<HTMLAudioElement>(null),
    resource = useRef(''),
    abort = useRef<AbortController | undefined>(undefined)
  const [loading, setLoading] = useState(false),
    [playing, setPlaying] = useState(false),
    [failed, setFailed] = useState(false)
  useEffect(() => {
    abort.current = new AbortController()
    const element = audio.current
    return () => {
      abort.current?.abort()
      element?.pause()
      if (resource.current) URL.revokeObjectURL(resource.current)
    }
  }, [])
  async function play() {
    try {
      setLoading(true)
      setFailed(false)
      if (!resource.current) {
        const a = await calls.access(projectId, speech.assetId, abort.current?.signal)
        if (a.audioHash !== speech.audioHash) throw new Error('Speech provenance changed')
        const response = await fetch(a.url, {
          credentials: 'include',
          cache: 'no-store',
          signal: abort.current?.signal,
        })
        if (!response.ok || !response.headers.get('content-type')?.startsWith('audio/mpeg'))
          throw new Error('Speech unavailable')
        const blob = await response.blob()
        if (blob.size !== a.bytes) throw new Error('Invalid speech size')
        const data = await blob.arrayBuffer()
        const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', data)), (n) =>
          n.toString(16).padStart(2, '0'),
        ).join('')
        if (hash !== speech.audioHash) throw new Error('Speech provenance changed')
        if (abort.current?.signal.aborted) return
        resource.current = URL.createObjectURL(blob)
        if (audio.current) audio.current.src = resource.current
      }
      if (!abort.current?.signal.aborted) await audio.current?.play()
    } catch {
      if (!abort.current?.signal.aborted) setFailed(true)
    } finally {
      if (!abort.current?.signal.aborted) setLoading(false)
    }
  }
  return (
    <div className="space-y-1">
      <audio
        ref={audio}
        preload="none"
        aria-hidden="true"
        onPlaying={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
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
        {t(playing ? 'dubbing.stop' : previous ? 'dubbing.playPrevious' : 'dubbing.play')}
      </Button>
      {failed && (
        <Typography variant="meta" role="alert">
          {t('dubbing.audioUnavailable')}
        </Typography>
      )}
    </div>
  )
}
