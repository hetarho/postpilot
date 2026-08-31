import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { PostImage } from '@/entities/image'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import type { ContentLanguage, PostContent } from '@/shared/api'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyText, type CopyFallbackElement } from '@/shared/lib'
import { Button, FieldLabel, SegmentedControl, Textarea, TextField } from '@/shared/ui'
import { EXPORT_FORMATS, type ExportFormat } from '../config/guidance'

interface ExportPanelProps {
  content: PostContent
  images: readonly PostImage[]
  createdAt: string
  contentLanguage: ContentLanguage
}

/** Four synchronous browser-only derivations of the canonical block array. */
export function ExportPanel({ content, images, createdAt, contentLanguage }: ExportPanelProps) {
  const { t } = useTranslation('posts')
  const [format, setFormat] = useState<ExportFormat>('naver')
  const [copiedTarget, setCopiedTarget] = useState<'output' | 'title'>()
  // Which control's copy fell back to manual selection, so its hint renders beside that control
  // rather than somewhere the user is not looking (§4.3).
  const [manualCopyTarget, setManualCopyTarget] = useState<'output' | 'title'>()
  const outputRef = useRef<HTMLTextAreaElement>(null)
  const titleRef = useRef<HTMLInputElement>(null)
  const feedbackTimer = useRef<number | undefined>(undefined)
  const copyGeneration = useRef(0)
  const copyQueue = useRef<Promise<void>>(Promise.resolve())
  const mounted = useRef(false)
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
  const formatOptions = EXPORT_FORMATS.map((value) => ({
    value,
    label: t(`export.formatLabel.${value}`),
  }))

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      copyGeneration.current += 1
      if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    }
  }, [])

  function invalidateCopyFeedback() {
    copyGeneration.current += 1
    setCopiedTarget(undefined)
    setManualCopyTarget(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
  }

  async function copy(
    target: 'output' | 'title',
    value: string,
    fallback: CopyFallbackElement | null,
  ) {
    const generation = ++copyGeneration.current
    const isCurrent = () =>
      mounted.current &&
      copyGeneration.current === generation &&
      fallback !== null &&
      fallback.value === value &&
      (target === 'output' ? outputRef.current === fallback : titleRef.current === fallback)
    setCopiedTarget(undefined)
    setManualCopyTarget(undefined)
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
    setManualCopyTarget(result.copied ? undefined : target)
    setCopiedTarget(result.copied ? target : undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    if (result.copied) {
      feedbackTimer.current = window.setTimeout(() => {
        setCopiedTarget(undefined)
      }, COPY_FEEDBACK_MS)
    }
  }

  // One line per copy target, mounted whether or not it has anything to say: a live region has to
  // exist BEFORE its text changes or a screen reader announces nothing, and the sole confirmation
  // used to be a 1.5s label swap on the button under the thumb that hid it.
  const titleStatus =
    copiedTarget === 'title'
      ? t('export.titleCopied')
      : manualCopyTarget === 'title'
        ? t('export.manualCopy')
        : ''
  const outputStatus =
    copiedTarget === 'output'
      ? t('action.copied', { ns: 'common' })
      : manualCopyTarget === 'output'
        ? t('export.manualCopy')
        : ''

  return (
    <section aria-labelledby="export-heading" className="mt-10">
      <h2 id="export-heading" className="text-lg font-semibold tracking-tight">
        {t('export.title')}
      </h2>
      {/* The four Korean format names measure ~380px in one row against 328px of content at 360px,
          which cut 마크다운 in half with no scrollbar to say so. Two columns at the base
          breakpoint fit all four; the strip comes back where the width exists. */}
      <SegmentedControl
        value={format}
        options={formatOptions}
        ariaLabel={t('export.format')}
        controls="export-output-panel"
        onChange={(next) => {
          invalidateCopyFeedback()
          setFormat(next)
        }}
        className="mt-4 grid grid-cols-2 sm:flex"
      />

      <div id="export-output-panel" role="tabpanel" className="mt-4">
        <p className="text-content-secondary text-sm">{t(`export.guidance.${format}`)}</p>

        {format === 'naver' && (
          <div className="mt-4">
            <FieldLabel htmlFor="export-title">{t('export.naverTitle')}</FieldLabel>
            <div className="mt-2 flex items-center gap-2">
              {/* `min-w-0` on the field, `shrink-0` on the button: the title is a server string
                  and must be the thing that gives way, never the control beside it (§8.5). */}
              <TextField
                id="export-title"
                ref={titleRef}
                value={content.title}
                readOnly
                className="min-w-0 flex-1"
              />
              <Button
                variant="secondary"
                className="shrink-0"
                onClick={() => void copy('title', content.title, titleRef.current)}
              >
                {t('export.copyTitle')}
              </Button>
            </div>
            <p role="status" className="text-content-tertiary mt-1 min-h-4 text-xs">
              {titleStatus}
            </p>
          </div>
        )}

        <FieldLabel htmlFor="export-output" className="sr-only">
          {t('export.result')}
        </FieldLabel>
        {/* The copy action sits ABOVE the output, not after it. This panel renders inside the
            editor, which already docks its own bar, and two docked bars in one scroller stick to
            the same offset and paint over each other — so §4.3's other option applies: put the
            action where the user already is. Above the field it shares a screen with the format
            tabs and the first lines of the result, which is what the user checks before copying;
            after an autoGrow field it would be a whole post's length away. */}
        <div className="mt-4 flex flex-col gap-2 sm:flex-row-reverse sm:items-center sm:justify-end sm:gap-3">
          {/* The label stays 복사: a swap to 복사됨 resizes the target under the thumb that just
              pressed it, and it was also the only signal that anything had happened (§6). */}
          <Button
            variant="cta"
            className="w-full sm:w-auto"
            onClick={() => void copy('output', output, outputRef.current)}
          >
            {t('action.copy', { ns: 'common' })}
          </Button>
          {/* Always mounted, never conditionally inserted: a live region that first appears WITH
              its text already in it is not announced (§9). */}
          <p role="status" className="text-content-tertiary min-h-4 text-xs">
            {outputStatus}
          </p>
        </div>
        {/* A Korean post runs to ~58 lines at this width. A fixed 18-row box scrolled internally,
            so every vertical swipe that landed on it moved the output instead of the page and the
            only place left to scroll was the 16px gutter (§4.4) — it grows instead. No `font-mono`:
            Tailwind's stock mono stack carries no Hangul, so every glyph fell back (§3). */}
        <Textarea
          id="export-output"
          ref={outputRef}
          value={output}
          readOnly
          spellCheck={false}
          rows={8}
          autoGrow
          className="mt-3 leading-relaxed"
        />
      </div>
    </section>
  )
}
