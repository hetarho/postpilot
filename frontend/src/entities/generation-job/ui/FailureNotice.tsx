import { Button } from '@/shared/ui'

export function FailureNotice({ error, onRetry }: { error: string; onRetry?: () => void }) {
  return (
    <div
      role="alert"
      className="bg-notice-danger-bg text-notice-danger-fg flex flex-wrap items-center gap-2 rounded-md px-3 py-2 text-sm"
    >
      <span>{error || '작업을 마치지 못했어요.'}</span>
      {onRetry && (
        <Button variant="ghost" onClick={onRetry} className="text-notice-danger-fg underline">
          다시 시도
        </Button>
      )}
    </div>
  )
}
