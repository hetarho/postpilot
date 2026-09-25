import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useSavePostContent, type ReplacementCandidate } from '@/entities/post'
import type { PostContent } from '@/shared/api'
import { attachContentQueue, type ContentQueueHandle, type ContentSaveState } from './content-queue'

export function useContentAutosave(args: {
  slug: string
  revision: bigint
  content: PostContent
  valid: boolean
  /** The replacement candidates the post holds at `revision`, read at attach time like the
   *  content. */
  candidates: readonly ReplacementCandidate[]
}): {
  state: ContentSaveState
  flush: () => Promise<bigint>
  /** Records a take: the next valid content this hook queues carries it (POST-79). */
  take: (candidate: ReplacementCandidate) => void
} {
  const save = useSavePostContent()
  const [state, setState] = useState<ContentSaveState>('idle')
  const queue = useRef<ContentQueueHandle | undefined>(undefined)
  const send = useRef(save.save)
  const opened = useRef({ content: args.content, candidates: args.candidates })
  // Takes made since the last queued snapshot; a snapshot that could not be queued (invalid
  // content) leaves them for the next one.
  const taken = useRef<ReplacementCandidate[]>([])
  useLayoutEffect(() => {
    send.current = save.save
    opened.current = { content: args.content, candidates: args.candidates }
  })
  useLayoutEffect(() => {
    const handle = attachContentQueue({
      slug: args.slug,
      revision: args.revision,
      saved: { content: opened.current.content, taken: [] },
      candidates: opened.current.candidates,
      send: (snapshot, revision, takenCandidates) =>
        send.current(args.slug, snapshot.content, revision, takenCandidates),
      onState: setState,
    })
    queue.current = handle
    setState(handle.state())
    return () => {
      handle.release()
      queue.current = undefined
    }
  }, [args.revision, args.slug])
  useLayoutEffect(() => {
    if (!args.valid || !queue.current) return
    queue.current.queue({ content: args.content, taken: taken.current })
    taken.current = []
  }, [args.content, args.valid])
  useEffect(() => {
    const flush = () => queue.current?.saveNow()
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') flush()
    }
    window.addEventListener('pagehide', flush)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      window.removeEventListener('pagehide', flush)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [])
  return {
    take: (candidate) => {
      taken.current = [...taken.current, candidate]
    },
    state: args.valid ? state : 'error',
    flush: () =>
      args.valid
        ? (queue.current?.flush() ?? Promise.reject(new Error('content editor is unavailable')))
        : Promise.reject(new Error('content is invalid')),
  }
}
