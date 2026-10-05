import { useEffect, useMemo, useRef, useState } from 'react'
import type { PostImage } from '@/entities/image'
import { imageByFile } from '@/entities/post'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import type { ContentLanguage, PostContent } from '@/shared/api'
import { blockPhotos, type CopyFallbackElement } from '@/shared/lib'
import type { ExportFormat } from '../config/guidance'
import { toHashtags } from '../lib/hashtags'
import { captionCopyStatus, captionFellBack, exportCopyStatus } from './copy-status'
import type { CopyTarget, TextCopyTarget } from './copy-target'
import { useCopyFeedback } from './useCopyFeedback'

/** The export panel's state (EXPORT-9, EXPORT-12, EXPORT-20, EXPORT-24): the format on screen,
 *  four synchronous browser-only derivations of the canonical block array, and every copy the
 *  panel offers with the status line each one reports. The panel renders what this hands it. */
export function useExportPanel({
  content,
  images,
  createdAt,
  contentLanguage,
}: {
  content: PostContent
  images: readonly PostImage[]
  createdAt: string
  contentLanguage: ContentLanguage
}) {
  const [format, setFormat] = useState<ExportFormat>('naver')
  const outputs = useMemo(
    () => ({
      naver: toNaver(content, images, contentLanguage),
      tistory: toTistory(content, images, contentLanguage),
      site: toSite(content, images, createdAt, contentLanguage),
      markdown: toMarkdown(content, images, createdAt, contentLanguage),
    }),
    [content, contentLanguage, createdAt, images],
  )
  const output = outputs[format]
  // Empty for a post with no usable tags, which is what keeps the field off the screen entirely
  // rather than mounting an empty control (THEME-29).
  const hashtags = toHashtags(content.tags)
  const outputWithTags = hashtags ? `${outputs.naver}\n\n${hashtags}` : ''
  // Photo numbers per block index, from the SAME canonical block array `toNaver` walks, so a photo
  // in the preview and its number in the copied text cannot drift apart: they match by position.
  // A single photo holds one number and a group one per photo (EXPORT-5, EXPORT-26). The number —
  // not the block index — is the copy target's identity; the number a person reads is it plus
  // one, because the text counts photos from 1.
  const photoNumbersByBlock = useMemo(() => {
    const map = new Map<number, number[]>()
    let next = 0
    content.blocks.forEach((block, index) => {
      const photos = blockPhotos(block)
      if (photos.length > 0)
        map.set(
          index,
          photos.map(() => next++),
        )
    })
    return map
  }, [content])
  const imagesByFilename = useMemo(() => imageByFile(images), [images])

  const feedback = useCopyFeedback(content)
  const { copied, manualCopy, photoFailure, outputRef, titleRef, tagsRef } = feedback
  const status = exportCopyStatus(feedback, {
    content,
    format,
    output,
    outputWithTags,
    hashtags,
  })
  const { outputFellBack } = status
  const copyButtonRef = useRef<HTMLButtonElement>(null)
  const copyWithTagsButtonRef = useRef<HTMLButtonElement>(null)

  // The manual fallback must be SEEN to be used: on the Naver tab the raw field mounts only after
  // the copy has fallen back, so `copyText` was handed no element and the selection happens here,
  // once the field exists. The dismissal side is the same effect's business: a content change
  // dissolves the fallback by derivation and UNMOUNTS the focused field, which would drop the
  // keyboard onto <body> — the focus is handed back to the copy button instead. A dismissal by
  // tab switch or by pressing another control leaves focus where the user put it.
  const fallbackWasRevealed = useRef<TextCopyTarget | undefined>(undefined)
  useEffect(() => {
    if (format === 'naver' && outputFellBack) {
      fallbackWasRevealed.current = manualCopy?.target
      outputRef.current?.focus()
      outputRef.current?.select()
      return
    }
    if (fallbackWasRevealed.current) {
      const target = fallbackWasRevealed.current
      fallbackWasRevealed.current = undefined
      if (document.activeElement === document.body) {
        if (target === 'outputWithTags') copyWithTagsButtonRef.current?.focus()
        else copyButtonRef.current?.focus()
      }
    }
  }, [format, manualCopy?.target, manualCopy?.value, outputFellBack, outputRef])

  return {
    format,
    /** A format switch drops every copy's feedback, and any copy still in flight with it. */
    selectFormat: (next: ExportFormat) => {
      feedback.invalidate()
      setFormat(next)
    },
    output,
    hashtags,
    outputWithTags,
    fallbackOutput: status.fallbackOutput,
    rawFieldVisible: status.rawFieldVisible,
    /** One status per text copy line; `undefined` says nothing. */
    status: {
      title: status.title,
      output: status.output,
      outputWithTags: status.outputWithTags,
      tags: status.tags,
    },
    hasPhotos: photoNumbersByBlock.size > 0,
    photoNumbersByBlock,
    imagesByFilename,
    outputRef,
    titleRef,
    tagsRef,
    copyButtonRef,
    copyWithTagsButtonRef,
    copyTitle: () => feedback.copy('title', content.title, titleRef.current),
    copyTags: () => feedback.copy('tags', hashtags, tagsRef.current),
    // `outputRef.current` is naturally null while the Naver tab shows the preview and the live
    // field on every other state — including a Naver retry from the revealed field.
    copyOutput: () =>
      feedback.copy(
        'output',
        output,
        outputRef.current?.value === output ? outputRef.current : null,
      ),
    copyOutputWithTags: () =>
      feedback.copy(
        'outputWithTags',
        outputWithTags,
        outputRef.current?.value === outputWithTags ? outputRef.current : null,
      ),
    /** One caption's status line and whether its field is revealed. */
    captionCopy: (marker: number, caption: string) => ({
      status: captionCopyStatus(feedback, content, marker, caption),
      fellBack: captionFellBack(feedback, content, marker, caption),
    }),
    copyCaption: (marker: number, caption: string) =>
      feedback.copy(`caption:${marker}`, caption, feedback.captionField(marker)),
    registerCaptionField: (marker: number, element: CopyFallbackElement | null) =>
      feedback.registerCaptionField(marker, element),
    photoCopied: (target: CopyTarget) => copied?.target === target,
    photoFailure: (target: CopyTarget) =>
      photoFailure?.target === target ? photoFailure.kind : undefined,
    copyPhoto: (target: CopyTarget, image: HTMLImageElement, rotation: number) =>
      feedback.copyPhoto(target, image, rotation),
  }
}
