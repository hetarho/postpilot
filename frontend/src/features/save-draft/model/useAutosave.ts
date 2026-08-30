import { useLayoutEffect, useEffect, useRef, useState } from 'react'
import { useSavePostDraft } from '@/entities/post'
import {
  type DraftQueueHandle,
  type SaveState,
  attachDraftQueue,
} from './draft-queue'

export interface UseAutosaveArgs {
  /** The post as the server last reported it, or undefined for a draft with no slug yet.
   *  It is the identity of this editing session — the editor is mounted per post. */
  post: { slug: string; title: string; memo: string; voice: { id: string } } | undefined
  /** What is in the inputs right now. */
  title: string
  memo: string
  /** The voice a draft with no post yet will be created in. Once the post exists its
   *  assignment changes only through `reassign` — never by this value moving — so a stale
   *  server value re-rendering the editor cannot undo a choice still in flight. */
  voiceId: string
  /** Called with the slug the first save minted. */
  onMinted?: (slug: string) => void
}

/** Saves the draft a beat after the user stops typing (PRD F-2 — no save button).
 *
 *  The hook is only the React end of it: the debounce, the retries and the in-flight
 *  bookkeeping live in the per-post queue (`draft-queue.ts`), which outlives this
 *  component on purpose. */
export function useAutosave({ post, title, memo, voiceId, onMinted }: UseAutosaveArgs): {
  state: SaveState
  /** The post's slug, creating the post first if it has none yet (see
   *  `DraftQueueHandle.mint`). For anything that needs a post to attach to — photos. */
  ensureSlug: () => Promise<string>
  /** Waits until the current title and memo are durably saved. */
  flush: () => Promise<void>
  /** Moves an existing post to another voice through the same queue as the text, so a
   *  title save still in flight cannot carry the old assignment over it. Resolves when the
   *  server holds the new voice; rejects with the server's answer when it refuses. */
  reassign: (voiceId: string) => Promise<void>
} {
  const slug = post?.slug
  const saveDraft = useSavePostDraft()
  const [state, setState] = useState<SaveState>('idle')
  const queueRef = useRef<DraftQueueHandle | undefined>(undefined)
  const sendRef = useRef(saveDraft.mutateAsync)
  const onMintedRef = useRef(onMinted)
  const postRef = useRef(post)
  const voiceRef = useRef(voiceId)

  // Layout effects throughout, not passive ones. A passive effect runs after paint and can
  // be deferred past a `pagehide` or a `visibilitychange`, and the keystroke it had not
  // recorded yet is exactly the one that would be lost.
  useLayoutEffect(() => {
    sendRef.current = saveDraft.mutateAsync
    onMintedRef.current = onMinted
    postRef.current = post
    voiceRef.current = voiceId
  })

  // Keyed by the slug alone, not by the post object: every successful save reseeds the
  // GetPost cache with a fresh `updated_at`, so keying on the object would tear the queue
  // down and rebuild it on each save — losing the reported state and firing the queued
  // text on every response, which is exactly what the debounce exists to prevent.
  useLayoutEffect(() => {
    const opened = postRef.current
    const handle = attachDraftQueue({
      slug: opened?.slug,
      saved: { title: opened?.title ?? '', memo: opened?.memo ?? '' },
      voiceId: opened?.voice.id ?? voiceRef.current,
      send: async (slug, draft, voiceId) => {
        const response = await sendRef.current({
          slug,
          title: draft.title,
          memo: draft.memo,
          voiceId,
        })
        // A 200 carrying no post is not a confirmation. Taking it as one would mark the
        // text saved, and for a draft with no slug yet would leave the next edit creating
        // a second post.
        if (!response.post?.slug) throw new Error('SavePostDraft returned no post')
        return response.post.slug
      },
      onState: setState,
      onMinted: (slug) => onMintedRef.current?.(slug),
    })
    queueRef.current = handle
    setState(handle.state())

    return () => {
      // Leaving must not cost the last second of typing. The queue keeps a failing save
      // alive after this component is gone, which is why release comes second.
      handle.saveNow()
      handle.release()
      queueRef.current = undefined
    }
  }, [slug])

  // Declared after the attach above, so the queue exists by the time the first text
  // arrives.
  useLayoutEffect(() => {
    queueRef.current?.queue({ title, memo })
  }, [title, memo])

  // Only a draft with no post yet follows the picker (see `UseAutosaveArgs.voiceId`).
  useLayoutEffect(() => {
    if (!postRef.current) void queueRef.current?.assignVoice(voiceId)
  }, [voiceId])

  useEffect(() => {
    const flush = () => queueRef.current?.saveNow()
    // `visibilitychange` is the one that fires while the page can still finish a request —
    // it is what a phone sends when the app goes to the background. `pagehide` is often
    // the last moment there is. Neither is a guarantee: a request started as the document
    // is discarded may be cut short, which is why the debounce is one second and not ten.
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
    state,
    ensureSlug: () =>
      queueRef.current?.mint() ?? Promise.reject(new Error('editor is not attached to a draft')),
    flush: () =>
      queueRef.current?.flush() ?? Promise.reject(new Error('editor is not attached to a draft')),
    reassign: (voiceId) =>
      queueRef.current?.assignVoice(voiceId) ??
      Promise.reject(new Error('editor is not attached to a draft')),
  }
}
