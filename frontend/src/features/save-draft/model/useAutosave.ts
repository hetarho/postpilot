import { useLayoutEffect, useEffect, useMemo, useRef, useState } from 'react'
import { isPublished, useSavePostDraft, type PostStatus } from '@/entities/post'
import { appFailureFromConnect, type ContentLanguage } from '@/shared/api'
import {
  attachDraftQueue,
  mergeAnswers,
  peekPendingDraft,
  type DraftQueueHandle,
  type SaveState,
  type TemplateAnswerDraft,
} from './draft-queue'

const NO_ANSWERS: readonly TemplateAnswerDraft[] = []

export interface UseAutosaveArgs {
  /** The post as the server last reported it, or undefined for a draft with no slug yet.
   *  It is the identity of this editing session — the editor is mounted per post. */
  post:
    | {
        slug: string
        title: string
        memo: string
        /** Decides the published lock (POST-86): the autosave reads it here, and nowhere else. */
        status: PostStatus
        voice: { id: string }
        template: { id: string }
        targetLanguage: ContentLanguage
        templateAnswers: TemplateAnswerDraft[]
      }
    | undefined
  /** The selected template's fields over these answers, in ①'s order; the draft carries exactly
   *  this (POST-62). Keep it stable per template, since it is a memo dependency. */
  answerPatch: (answers: readonly TemplateAnswerDraft[]) => TemplateAnswerDraft[]
  /** The voice a draft with no post yet will be created in. Once the post exists its
   *  assignment changes only through `reassign` — never by this value moving — so a stale
   *  server value re-rendering the editor cannot undo a choice still in flight. */
  voiceId: string
  /** The 템플릿 a draft with no post yet will be created with, '' for 없음. Same rule as
   *  `voiceId`: once the post exists its assignment changes only through `assignTemplate`. */
  templateId: string
  /** The next full-write language; local-only until an unsaved draft is first created. */
  targetLanguage: ContentLanguage
  /** Called with the slug the first save minted. */
  onMinted?: (slug: string) => void
}

/** ①'s text as the autosave holds it, and the draft's save controls. */
export interface Autosave {
  state: SaveState
  /** The text ① shows: the user's, or the server's while the post is published. */
  title: string
  memo: string
  /** The selected template's fields' answers, as the draft carries them (POST-62). */
  answers: TemplateAnswerDraft[]
  /** Each does nothing while the post is published. `setAnswers` takes ①'s fields as the user
   *  left them, a whole patch by label. */
  setTitle: (title: string) => void
  setMemo: (memo: string) => void
  setAnswers: (answers: TemplateAnswerDraft[]) => void
  /** The post's slug, creating the post first if it has none yet (see
   *  `DraftQueueHandle.mint`). For anything that needs a post to attach to — photos. */
  ensureSlug: () => Promise<string>
  /** Waits until the current title and memo are durably saved. */
  flush: () => Promise<void>
  /** Moves an existing post to another voice through the same queue as the text, so a
   *  title save still in flight cannot carry the old assignment over it. Resolves when the
   *  server holds the new voice; rejects with the server's answer when it refuses. */
  reassign: (voiceId: string) => Promise<void>
  /** Assigns or clears ('') an existing post's 템플릿 through the same queue as the text, so a
   *  title save still in flight cannot carry the old assignment over a newer selection. */
  assignTemplate: (templateId: string) => Promise<void>
  assignTargetLanguage: (language: ContentLanguage) => Promise<void>
}

/** Saves the draft a beat after the user stops typing (PRD F-2 — no save button), and owns ①'s
 *  text while it does.
 *
 *  The hook is only the React end of it: the debounce, the retries and the in-flight
 *  bookkeeping live in the per-post queue (`draft-queue.ts`), which outlives this
 *  component on template.
 *
 *  It decides the published lock (POST-86) itself: while the post is published it queues
 *  nothing, shows the server's text and ignores edits. A save refused as published takes the
 *  text back all the way to the screen, so nothing the lock refused is shown or queued again,
 *  and a post that reopens saves again from the server's values. */
export function useAutosave({
  post,
  answerPatch,
  voiceId,
  templateId,
  targetLanguage,
  onMinted,
}: UseAutosaveArgs): Autosave {
  const slug = post?.slug
  const locked = post ? isPublished(post) : false
  // Text still queued for this post outranks what the server reported: it is what the
  // previous editor was in the middle of saving when the mint moved the URL, so it is
  // newer by exactly the characters typed during that round trip.
  const [local, setLocal] = useState(() => {
    const opening = post ? (peekPendingDraft(post.slug) ?? post) : undefined
    return {
      title: opening?.title ?? '',
      memo: opening?.memo ?? '',
      edits: NO_ANSWERS as readonly TemplateAnswerDraft[],
    }
  })
  const stored = post?.templateAnswers
  // The stored answers with the local edits laid over them; the server's alone while locked.
  const source = useMemo(
    () => (locked ? (stored ?? NO_ANSWERS) : mergeAnswers(stored ?? NO_ANSWERS, local.edits)),
    [locked, stored, local.edits],
  )
  const answers = useMemo(() => answerPatch(source), [answerPatch, source])
  const title = locked && post ? post.title : local.title
  const memo = locked && post ? post.memo : local.memo
  const saveDraft = useSavePostDraft()
  const [state, setState] = useState<SaveState>('idle')
  const queueRef = useRef<DraftQueueHandle | undefined>(undefined)
  const sendRef = useRef(saveDraft.save)
  const onMintedRef = useRef(onMinted)
  const postRef = useRef(post)
  const voiceRef = useRef(voiceId)
  const templateRef = useRef(templateId)
  const targetLanguageRef = useRef(targetLanguage)

  // Layout effects throughout, not passive ones. A passive effect runs after paint and can
  // be deferred past a `pagehide` or a `visibilitychange`, and the keystroke it had not
  // recorded yet is exactly the one that would be lost.
  useLayoutEffect(() => {
    sendRef.current = saveDraft.save
    onMintedRef.current = onMinted
    postRef.current = post
    voiceRef.current = voiceId
    templateRef.current = templateId
    targetLanguageRef.current = targetLanguage
  })

  // Keyed by the slug alone, not by the post object: every successful save reseeds the
  // GetPost cache with a fresh `updated_at`, so keying on the object would tear the queue
  // down and rebuild it on each save — losing the reported state and firing the queued
  // text on every response, which is exactly what the debounce exists to prevent.
  useLayoutEffect(() => {
    const opened = postRef.current
    const handle = attachDraftQueue({
      slug: opened?.slug,
      // The stored answers are the baseline like the title and the memo are, or the fields
      // would read as dirty the moment the editor mounted.
      saved: {
        title: opened?.title ?? '',
        memo: opened?.memo ?? '',
        answers: opened?.templateAnswers ?? [],
      },
      voiceId: opened?.voice.id ?? voiceRef.current,
      templateId: opened?.template.id ?? templateRef.current,
      targetLanguage: opened?.targetLanguage ?? targetLanguageRef.current,
      send: ({ slug, draft, voiceId, templateId, targetLanguage }) =>
        sendRef.current({
          slug,
          title: draft.title,
          memo: draft.memo,
          // Every entry is an upsert of that label, so the whole current set goes with any
          // save that carries anything at all (POST-62).
          templateAnswers: draft.answers,
          voiceId,
          templateId,
          targetLanguage,
        }),
      // A post published in another tab refuses every save the same way (POST-86), so the text is
      // taken back rather than retried, and the post's refetch re-renders ① locked.
      retry: (cause) => appFailureFromConnect(cause).reason !== 'POST_PUBLISHED_LOCKED',
      onState: setState,
      onMinted: (slug) => onMintedRef.current?.(slug),
      // The post as it stands when the refusal lands is the pre-refetch one, whose title and memo
      // are the server's: a publish changes neither.
      onTakenBack: () => {
        const current = postRef.current
        setLocal({ title: current?.title ?? '', memo: current?.memo ?? '', edits: NO_ANSWERS })
      },
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
  // The answers are compared by value, so a new array with the same contents on every render
  // costs nothing but a comparison. A published post queues nothing at all.
  useLayoutEffect(() => {
    if (locked) return
    queueRef.current?.queue({ title, memo, answers })
  }, [locked, title, memo, answers])

  // Only a draft with no post yet follows the picker (see `UseAutosaveArgs.voiceId`).
  useLayoutEffect(() => {
    if (!postRef.current) void queueRef.current?.assign('voiceId', voiceId)
  }, [voiceId])

  useLayoutEffect(() => {
    if (!postRef.current) void queueRef.current?.assign('templateId', templateId)
  }, [templateId])

  useLayoutEffect(() => {
    if (!postRef.current) void queueRef.current?.assign('targetLanguage', targetLanguage)
  }, [targetLanguage])

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

  // A resolve means "the server holds it" in the queue's contract, so a locked assignment
  // rejects rather than resolving without sending. No screen offers one: the controls are off.
  const refuseLocked = () => Promise.reject(new Error('post is published'))
  return {
    state,
    title,
    memo,
    answers,
    setTitle: (next) => {
      if (!locked) setLocal((current) => ({ ...current, title: next }))
    },
    setMemo: (next) => {
      if (!locked) setLocal((current) => ({ ...current, memo: next }))
    },
    setAnswers: (next) => {
      if (!locked) setLocal((current) => ({ ...current, edits: next }))
    },
    ensureSlug: () =>
      queueRef.current?.mint() ?? Promise.reject(new Error('editor is not attached to a draft')),
    flush: () =>
      queueRef.current?.flush() ?? Promise.reject(new Error('editor is not attached to a draft')),
    reassign: (voiceId) =>
      locked
        ? refuseLocked()
        : (queueRef.current?.assign('voiceId', voiceId) ??
          Promise.reject(new Error('editor is not attached to a draft'))),
    assignTemplate: (templateId) =>
      locked
        ? refuseLocked()
        : (queueRef.current?.assign('templateId', templateId) ??
          Promise.reject(new Error('editor is not attached to a draft'))),
    assignTargetLanguage: (language) =>
      locked
        ? refuseLocked()
        : (queueRef.current?.assign('targetLanguage', language) ??
          Promise.reject(new Error('editor is not attached to a draft'))),
  }
}
