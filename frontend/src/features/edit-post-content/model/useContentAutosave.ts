import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useSavePostContent } from '@/entities/post'
import type { PostContent } from '@/shared/api'
import {
  attachContentQueue,
  type ContentQueueHandle,
  type ContentSaveState,
} from './content-queue'

export function useContentAutosave(args: {
  slug: string
  revision: bigint
  content: PostContent
  targetLength: number
  valid: boolean
}): { state: ContentSaveState; flush: () => Promise<void> } {
  const save = useSavePostContent()
  const [state, setState] = useState<ContentSaveState>('idle')
  const queue = useRef<ContentQueueHandle | undefined>(undefined)
  const send = useRef(save.save)
  const opened = useRef({ content: args.content, targetLength: args.targetLength })
  useLayoutEffect(() => {
    send.current = save.save
    opened.current = { content: args.content, targetLength: args.targetLength }
  })
  useLayoutEffect(() => {
    const handle = attachContentQueue({
      slug: args.slug,
      revision: args.revision,
      saved: opened.current,
      send: (snapshot, revision) =>
        send.current(args.slug, snapshot.content, revision, snapshot.targetLength),
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
    if (args.valid)
      queue.current?.queue({ content: args.content, targetLength: args.targetLength })
  }, [args.content, args.targetLength, args.valid])
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
    state: args.valid ? state : 'error',
    flush: () =>
      args.valid
        ? (queue.current?.flush() ?? Promise.reject(new Error('content editor is unavailable')))
        : Promise.reject(new Error('content is invalid')),
  }
}
