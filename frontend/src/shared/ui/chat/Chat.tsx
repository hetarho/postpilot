import { useEffect, useRef, type FormEvent, type ReactNode, type Ref } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { Button } from '../button/Button'
import { Textarea } from '../field/Textarea'

export interface ChatMessage {
  id: string
  sender: 'partner' | 'owner'
  content: ReactNode
}

/** A conversation's presentation only. Its caller owns messages and every submission. */
export function Chat({
  messages,
  label,
  partnerLabel,
  ownerLabel,
  composer,
  className,
}: {
  messages: readonly ChatMessage[]
  label: string
  partnerLabel: string
  ownerLabel: string
  composer?: ReactNode
  className?: string
}) {
  const end = useRef<HTMLDivElement>(null)
  const previousCount = useRef(messages.length)
  useEffect(() => {
    if (messages.length > previousCount.current) end.current?.scrollIntoView?.({ block: 'nearest' })
    previousCount.current = messages.length
  }, [messages.length])
  return (
    <div className={twMerge('flex min-w-0 flex-col gap-4', className)}>
      <div role="log" aria-label={label} aria-live="polite" aria-relevant="additions text">
        <ol className="space-y-4">
          {messages.map((message) => (
            <li
              key={message.id}
              className={clsx(
                'flex min-w-0',
                message.sender === 'owner' ? 'justify-end pl-6' : 'justify-start pr-6',
              )}
            >
              <div
                className={clsx(
                  'min-w-0 rounded-lg px-4 py-3 text-sm break-words',
                  message.sender === 'owner' ? 'bg-surface-raised' : 'bg-surface-recessed',
                )}
              >
                <p className="text-content-secondary mb-1 text-xs">
                  {message.sender === 'owner' ? ownerLabel : partnerLabel}
                </p>
                <div className="whitespace-pre-wrap">{message.content}</div>
              </div>
            </li>
          ))}
        </ol>
        <div ref={end} />
      </div>
      {composer}
    </div>
  )
}

export function ChatComposer({
  value,
  onChange,
  onSubmit,
  label,
  sendLabel,
  pending = false,
  disabled = false,
  invalid = false,
  textareaRef,
  attachment,
  feedback,
  actions,
}: {
  value: string
  onChange: (value: string) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  label: string
  sendLabel: string
  pending?: boolean
  disabled?: boolean
  invalid?: boolean
  textareaRef?: Ref<HTMLTextAreaElement>
  attachment?: ReactNode
  feedback?: ReactNode
  actions?: ReactNode
}) {
  return (
    <form
      onSubmit={onSubmit}
      className="bg-surface-base pb-safe-b sticky bottom-0 z-10 min-w-0 space-y-3 pt-3"
    >
      {attachment}
      <div className="flex min-w-0 flex-wrap items-end gap-2">
        <Textarea
          ref={textareaRef}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          aria-label={label}
          aria-invalid={invalid || undefined}
          disabled={pending}
          autoGrow
          rows={1}
          className="max-h-field min-h-11 min-w-0 flex-1 basis-40"
        />
        <Button
          type="submit"
          variant="cta"
          pending={pending}
          disabled={disabled || pending}
          className="ml-auto min-h-11 max-w-full min-w-11 shrink-0"
        >
          {sendLabel}
        </Button>
      </div>
      {feedback}
      {actions && <div className="flex flex-wrap items-center justify-end gap-2">{actions}</div>}
    </form>
  )
}
