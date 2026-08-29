import { useEffect, useMemo, useRef, useState } from 'react'
import type { PostImage } from '@/entities/image'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import type { PostContent } from '@/shared/api'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyText, type CopyFallbackElement } from '@/shared/lib'
import { ActionBar, Button, FieldLabel, SegmentedControl, Textarea, TextField } from '@/shared/ui'
import { EXPORT_FORMATS, EXPORT_GUIDANCE, type ExportFormat } from '../config/guidance'

const MANUAL_COPY_HINT = '자동 복사가 막혀 있어요 — 선택된 텍스트를 길게 눌러 복사하세요'

interface ExportPanelProps {
  content: PostContent
  images: readonly PostImage[]
  createdAt: string
}

/** Four synchronous browser-only derivations of the canonical block array. */
export function ExportPanel({ content, images, createdAt }: ExportPanelProps) {
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
      naver: toNaver(content, images),
      tistory: toTistory(content, images),
      site: toSite(content, images, createdAt),
      markdown: toMarkdown(content, images, createdAt),
    }),
    [content, createdAt, images],
  )
  const output = outputs[format]

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
      ? '제목이 복사됐어요'
      : manualCopyTarget === 'title'
        ? MANUAL_COPY_HINT
        : ''
  const outputStatus =
    copiedTarget === 'output' ? '복사됨' : manualCopyTarget === 'output' ? MANUAL_COPY_HINT : ''

  return (
    <section aria-labelledby="export-heading" className="mt-10">
      <h2 id="export-heading" className="text-lg font-semibold tracking-tight">
        내보내기
      </h2>
      {/* The four Korean format names measure ~380px in one row against 328px of content at 360px,
          which cut 마크다운 in half with no scrollbar to say so. Two columns at the base
          breakpoint fit all four; the strip comes back where the width exists. */}
      <SegmentedControl
        value={format}
        options={EXPORT_FORMATS}
        ariaLabel="내보내기 형식"
        controls="export-output-panel"
        onChange={(next) => {
          invalidateCopyFeedback()
          setFormat(next)
        }}
        className="mt-4 grid grid-cols-2 sm:flex"
      />

      <div id="export-output-panel" role="tabpanel" className="mt-4">
        <p className="text-content-secondary text-sm">{EXPORT_GUIDANCE[format]}</p>

        {format === 'naver' && (
          <div className="mt-4">
            <FieldLabel htmlFor="export-title">네이버 제목</FieldLabel>
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
                제목 복사
              </Button>
            </div>
            <p role="status" className="text-content-tertiary mt-1 min-h-4 text-xs">
              {titleStatus}
            </p>
          </div>
        )}

        <FieldLabel htmlFor="export-output" className="sr-only">
          내보내기 결과
        </FieldLabel>
        {/* A Korean post runs to ~58 lines at this width. A fixed 18-row box scrolled internally,
            so every vertical swipe that landed on it moved the output instead of the page and the
            only place left to scroll was the 16px gutter (§4.4) — it grows instead, and the copy
            action docks so it never leaves the screen the output is on. No `font-mono`: Tailwind's
            stock mono stack carries no Hangul, so every Korean glyph fell back per glyph (§3). */}
        <Textarea
          id="export-output"
          ref={outputRef}
          value={output}
          readOnly
          spellCheck={false}
          rows={8}
          autoGrow
          className="mt-4 leading-relaxed"
        />
      </div>

      <ActionBar ariaLabel="내보내기 동작">
        <p role="status" className="text-content-tertiary min-h-4 text-xs">
          {outputStatus}
        </p>
        {/* The label stays 복사: a swap to 복사됨 resizes the target under the thumb that just
            pressed it, and it was also the only signal that anything had happened (§6). */}
        <Button
          variant="cta"
          className="mt-2 w-full sm:w-auto"
          onClick={() => void copy('output', output, outputRef.current)}
        >
          복사
        </Button>
      </ActionBar>
    </section>
  )
}
