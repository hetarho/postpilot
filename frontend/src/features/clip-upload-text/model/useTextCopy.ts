import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyText } from '@/shared/lib'

/** A shared queue survives channel changes. Feedback and manual selection belong to the
 * exact content on screen, with a new authority even when an earlier value returns. */
export function useTextCopy(queueRef: RefObject<Promise<void>>, identity: string) {
  const mounted = useRef(false)
  const generationRef = useRef(0)
  const timer = useRef<number | undefined>(undefined)
  const fieldRef = useRef<HTMLTextAreaElement>(null)
  const buttonRefs = useRef(new Map<string, HTMLButtonElement>())
  const targetRef = useRef('body')
  const [content, setContent] = useState(identity)
  const [copied, setCopied] = useState<string>()
  const [manual, setManual] = useState<{ target: string; value: string }>()
  if (content !== identity) {
    setContent(identity)
    setCopied(undefined)
    setManual(undefined)
  }
  useLayoutEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      generationRef.current += 1
      window.clearTimeout(timer.current)
    }
  }, [identity])
  const registerField = useCallback((element: HTMLTextAreaElement | null) => {
    const previous = fieldRef.current
    if (!element && previous && document.activeElement === previous) {
      buttonRefs.current.get(targetRef.current)?.focus()
    }
    fieldRef.current = element
  }, [])
  const registerButton = (target: string, element: HTMLButtonElement | null) => {
    if (element) buttonRefs.current.set(target, element)
    else buttonRefs.current.delete(target)
  }
  const copy = async (target: string, value: string) => {
    const own = ++generationRef.current
    const current = () => mounted.current && generationRef.current === own
    targetRef.current = target
    setCopied(undefined)
    setManual(undefined)
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
  return { copy, copied, manual, registerField, registerButton }
}
