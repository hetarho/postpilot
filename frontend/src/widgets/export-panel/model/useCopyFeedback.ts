import { useEffect, useRef, useState } from 'react'
import type { PostContent } from '@/shared/api'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyImage, copyText, type CopyFallbackElement, type CopyImageResult } from '@/shared/lib'
import {
  captionMarker,
  type CopiedFeedback,
  type CopyTarget,
  type ManualCopy,
  type PhotoFailure,
  type TextCopyTarget,
} from './copy-target'

/** Every copy the export panel makes, on one discipline: one generation counter so a stale async
 *  result cannot land, one queue so two presses do not race for the clipboard, one
 *  `COPY_FEEDBACK_MS` dwell for a confirmation, and the manual fallback a refused text copy
 *  leaves behind (EXPORT-12, EXPORT-24).
 *
 *  It owns the fallback FIELDS too, as refs the panel attaches: a text copy's result is current
 *  only while the field it would select is still that copy's field, and the fields are read LIVE
 *  because one that unmounted mid-copy must stop matching. `content` is the identity a fallback
 *  is recorded against. */
export function useCopyFeedback(content: PostContent) {
  // A photo target names the marker it belongs to, so two markers for one file still report
  // separately and the confirmation lands on the entry that was pressed. A TEXT copy stores the
  // value that reached the clipboard beside it: the Naver tab's copy carries no fallback element
  // whose value `isCurrent` could compare, so without this a copy racing a content change could
  // announce 복사됨 for a body the post no longer contains — the value comparison in the status
  // derivations is what drops that stale confirmation.
  const [copied, setCopied] = useState<CopiedFeedback>()
  // Which control's copy fell back to manual selection, so its hint renders beside that control
  // rather than somewhere the user is not looking (THEME-24). On the Naver tab this also REVEALS the
  // raw marker text: its default view is the rendered post, and a selection needs a text field.
  const [manualCopy, setManualCopy] = useState<ManualCopy>()
  // Per-photo failure kind, keyed the same way. It is separate from `manualCopy` because a
  // photo has no manual fallback at all — there is nothing to select and hold (see `copyImage`).
  const [photoFailure, setPhotoFailure] = useState<PhotoFailure>()
  const outputRef = useRef<HTMLTextAreaElement>(null)
  const titleRef = useRef<HTMLInputElement>(null)
  const tagsRef = useRef<HTMLInputElement>(null)
  // Each revealed caption field, by marker number. A map rather than a ref per caption because
  // the number of photos is the post's business, and it is read LIVE for the same reason the
  // three refs above are: a field that unmounted mid-copy must stop matching.
  const captionFields = useRef(new Map<number, CopyFallbackElement>())
  const feedbackTimer = useRef<number | undefined>(undefined)
  const copyGeneration = useRef(0)
  const copyQueue = useRef<Promise<void>>(Promise.resolve())
  const mounted = useRef(false)

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      copyGeneration.current += 1
      if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    }
  }, [])

  /** Drops every confirmation, fallback and photo failure, and any result still in flight. */
  function invalidate() {
    copyGeneration.current += 1
    setCopied(undefined)
    setManualCopy(undefined)
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
  }

  /** The image copy, on the SAME discipline as the text copy below: one generation counter so a
   *  stale async result cannot land, one queue so two presses do not race for the clipboard, the
   *  same `COPY_FEEDBACK_MS` dwell, and the same always-mounted live region. It reports the
   *  failure KIND instead of a manual-selection hint, because an image has no manual fallback. */
  async function copyPhoto(target: CopyTarget, image: HTMLImageElement, rotation: number) {
    const generation = ++copyGeneration.current
    const isCurrent = () => mounted.current && copyGeneration.current === generation
    setCopied(undefined)
    setManualCopy(undefined)
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)

    // The result is READ OUT of the chained promise rather than written into a mutable outer
    // variable the way the text copy does: an image copy answers with a kind, and a `let` holding
    // one narrows to the literal it was initialized with.
    const operation = copyQueue.current
      .then(() => copyImage(image, rotation))
      .catch((): CopyImageResult => ({ kind: 'unreadable' }))
    copyQueue.current = operation.then(() => undefined)
    const result = await operation
    if (!isCurrent()) return
    if (result.kind === 'copied') {
      setCopied({ target })
      feedbackTimer.current = window.setTimeout(() => setCopied(undefined), COPY_FEEDBACK_MS)
      return
    }
    setPhotoFailure({ target, kind: result.kind })
  }

  /** `fallback` is null when the manual field is not mounted yet (the Naver preview): the copy is
   *  still attempted, and a refusal reveals the field — the panel then selects it. */
  async function copy(target: TextCopyTarget, value: string, fallback: CopyFallbackElement | null) {
    const generation = ++copyGeneration.current
    // Looked up per target, not chosen by a two-way ternary: with three text copies a ternary
    // would compare a tags copy against the TITLE field's element and report a stale copy as
    // current. Read through the REF, not snapshotted from it here: the tags field is
    // conditionally mounted, so a result settling after it unmounted has to compare against the
    // ref as it stands now — a snapshot would keep matching a detached input.
    const fieldOf = (of: TextCopyTarget): CopyFallbackElement | null => {
      switch (of) {
        case 'output':
        case 'outputWithTags':
          return outputRef.current
        case 'title':
          return titleRef.current
        case 'tags':
          return tagsRef.current
        default:
          return captionFields.current.get(captionMarker(of)) ?? null
      }
    }
    const isCurrent = () =>
      mounted.current &&
      copyGeneration.current === generation &&
      (fallback === null || (fallback.value === value && fieldOf(target) === fallback))
    setCopied(undefined)
    // `manualCopy` is NOT cleared up front the way the other feedback is: on the Naver tab it is
    // what keeps the revealed fallback field mounted, and clearing it here would unmount the field
    // the user is retrying from for the whole in-flight wait. The outcome below overwrites it.
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)

    let result = { copied: false }
    const operation = copyQueue.current
      .then(async () => {
        result = await copyText(value, fallback, isCurrent)
      })
      .catch(() => {
        result = { copied: false }
      })
    copyQueue.current = operation
    await operation
    if (!isCurrent()) return
    setManualCopy(result.copied ? undefined : { target, value, source: content })
    setCopied(result.copied ? { target, value } : undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    if (result.copied) {
      feedbackTimer.current = window.setTimeout(() => {
        setCopied(undefined)
      }, COPY_FEEDBACK_MS)
    }
  }

  return {
    copied,
    manualCopy,
    photoFailure,
    outputRef,
    titleRef,
    tagsRef,
    /** The revealed field of one caption, or null while its copy has not fallen back. */
    captionField: (marker: number) => captionFields.current.get(marker) ?? null,
    registerCaptionField: (marker: number, element: CopyFallbackElement | null) => {
      if (element) captionFields.current.set(marker, element)
      else captionFields.current.delete(marker)
    },
    copy,
    copyPhoto,
    invalidate,
  }
}
