import { Button, Notice } from '@/shared/ui'

export function FailureNotice({ error, onRetry }: { error: string; onRetry?: () => void }) {
  return (
    <Notice tone="danger" role="alert">
      {/* The message is the provider's own string, so it can end in an unbreakable 60-character
          URL. Without `min-w-0` a flex item's automatic minimum is its min-content width, and one
          such token drags the notice — and the document with it — into horizontal scroll (§8.5). */}
      <span className="min-w-0 break-words">{error || '작업을 마치지 못했어요.'}</span>
      {onRetry && (
        <Button
          variant="ghost"
          onClick={onRetry}
          className="text-notice-danger-fg shrink-0 underline"
        >
          다시 시도
        </Button>
      )}
    </Notice>
  )
}
