import { useEffect, useMemo, useRef, useState } from 'react'
import type { PostImage } from '@/entities/image'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import type { PostContent } from '@/shared/api'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyText, type CopyFallbackElement } from '@/shared/lib'
import { Button, FieldLabel, Textarea, TextField } from '@/shared/ui'
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
  const [manualCopy, setManualCopy] = useState(false)
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
    setManualCopy(false)
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
    setManualCopy(false)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)

    let result = { copied: false }
    const operation = copyQueue.current
      .then(async () => {
        result = await copyText(
          value,
          fallback,
          isCurrent,
        )
      })
      .catch(() => {
        result = { copied: false }
      })
    copyQueue.current = operation
    await operation
    if (!isCurrent()) return
    setManualCopy(!result.copied)
    setCopiedTarget(result.copied ? target : undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    if (result.copied) {
      feedbackTimer.current = window.setTimeout(() => {
        setCopiedTarget(undefined)
      }, COPY_FEEDBACK_MS)
    }
  }

  return (
    <section aria-labelledby="export-heading" className="mt-10 pb-16">
      <h2 id="export-heading" className="text-lg font-semibold tracking-tight">
        내보내기
      </h2>
      <div role="tablist" aria-label="내보내기 형식" className="mt-4 flex gap-1 overflow-x-auto">
        {EXPORT_FORMATS.map((item) => (
          <Button
            key={item.key}
            role="tab"
            variant={format === item.key ? 'secondary' : 'ghost'}
            aria-selected={format === item.key}
            aria-controls="export-output-panel"
            onClick={() => {
              invalidateCopyFeedback()
              setFormat(item.key)
            }}
            className="shrink-0"
          >
            {item.label}
          </Button>
        ))}
      </div>

      <div id="export-output-panel" role="tabpanel" className="mt-4">
        <p className="text-content-secondary text-sm">{EXPORT_GUIDANCE[format]}</p>

        {format === 'naver' && (
          <div className="mt-4">
            <FieldLabel htmlFor="export-title">네이버 제목</FieldLabel>
            <div className="mt-2 flex items-center gap-2">
              <TextField id="export-title" ref={titleRef} value={content.title} readOnly />
              <Button
                variant="secondary"
                className="shrink-0"
                onClick={() => void copy('title', content.title, titleRef.current)}
              >
                {copiedTarget === 'title' ? '복사됨' : '제목 복사'}
              </Button>
            </div>
          </div>
        )}

        <FieldLabel htmlFor="export-output" className="sr-only">
          내보내기 결과
        </FieldLabel>
        <Textarea
          id="export-output"
          ref={outputRef}
          value={output}
          readOnly
          spellCheck={false}
          rows={18}
          className="mt-4 resize-y font-mono text-xs leading-relaxed"
        />
        <div className="mt-3 flex items-center gap-3">
          <Button variant="cta" onClick={() => void copy('output', output, outputRef.current)}>
            {copiedTarget === 'output' ? '복사됨' : '복사'}
          </Button>
          {manualCopy && (
            <p role="status" className="text-content-tertiary text-xs">
              {MANUAL_COPY_HINT}
            </p>
          )}
        </div>
      </div>
    </section>
  )
}
