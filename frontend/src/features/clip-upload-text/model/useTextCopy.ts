import { useEffect, useRef, useState, type RefObject } from 'react'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyText } from '@/shared/lib'

/** The parent retains the queue across channel/content changes. This keyed view only owns
 * feedback for the exact text on screen, with a fresh authority on every mount. */
export function useTextCopy(queueRef: RefObject<Promise<void>>) {
  const mounted = useRef(false)
  const generationRef = useRef(0)
  const timer = useRef<number | undefined>(undefined)
  const fieldRef = useRef<HTMLTextAreaElement>(null)
  const [copied, setCopied] = useState<string>()
  const [manual, setManual] = useState<{ target: string; value: string }>()
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      generationRef.current += 1
      window.clearTimeout(timer.current)
    }
  }, [])
  const copy = async (target: string, value: string) => {
    const own = ++generationRef.current
    const current = () => mounted.current && generationRef.current === own
    setCopied(undefined)
    window.clearTimeout(timer.current)
    const operation = queueRef.current
      .then(async () => {
        if (!current()) return undefined
        return copyText(value, null, current)
      })
      .catch(() => ({ copied: false }))
    queueRef.current = operation.then(() => undefined)
    const result = await operation
    if (!current() || !result) return
    setCopied(result.copied ? target : undefined)
    setManual(result.copied ? undefined : { target, value })
    if (result.copied)
      timer.current = window.setTimeout(() => setCopied(undefined), COPY_FEEDBACK_MS)
  }
  useEffect(() => {
    if (!manual) return
    fieldRef.current?.focus()
    fieldRef.current?.select()
  }, [manual])
  return { copy, copied, manual, fieldRef }
}
