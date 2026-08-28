import type { ReactNode } from 'react'
import { type PostImage, Thumbnail } from '@/entities/image'
import { Button } from '@/shared/ui'
import type { UploadFailure, UploadItem } from '../model/upload-batch'

interface PhotoStripProps {
  /** The post's photos — the server's, plus the ones this client has confirmed. */
  images: readonly PostImage[]
  /** This session's uploads for the post that are not photos yet. */
  items: readonly UploadItem[]
  onDelete: (image: PostImage) => void
  deletingId?: string
  deleteFailedId?: string
  onRetry: (id: string) => void
  onDismiss: (id: string) => void
}

const FAILURE_LABELS: Record<UploadFailure, string> = {
  network: '올리지 못했어요',
  'duplicate-filename': '같은 이름의 사진이 이미 있어요',
  invalid: '서버가 사진을 거절했어요',
}

const STATUS_LABELS: Partial<Record<UploadItem['status'], string>> = {
  selected: '대기 중',
  converting: '변환 중',
  uploading: '올리는 중',
  confirming: '올리는 중',
}

/** Saved photos first, then the ones still on their way. */
export function PhotoStrip({
  images,
  items,
  onDelete,
  deletingId,
  deleteFailedId,
  onRetry,
  onDismiss,
}: PhotoStripProps) {
  const inFlight = items.filter((item) => item.status !== 'skipped')
  if (images.length === 0 && inFlight.length === 0) return null

  return (
    <ul className="flex gap-2 overflow-x-auto pb-1" aria-label="사진">
      {images.map((image) => (
        <li key={image.id}>
          <Thumbnail src={image.viewUrl} alt={image.filename} dimmed={deletingId === image.id}>
            <Button
              variant="danger"
              size="icon"
              onClick={() => onDelete(image)}
              disabled={deletingId === image.id}
              aria-label={`${image.filename} 삭제`}
              className="bg-media-scrim-bg hover:bg-media-scrim-bg active:bg-media-scrim-bg absolute top-1 right-1"
            >
              <span aria-hidden="true">×</span>
            </Button>
            {deleteFailedId === image.id && <Overlay role="alert">삭제하지 못했어요</Overlay>}
          </Thumbnail>
        </li>
      ))}
      {inFlight.map((item) => (
        <li key={item.id}>
          <Thumbnail src={item.previewUrl} alt={item.filename} dimmed>
            {item.status === 'failed' ? (
              <Overlay role="alert">
                <span>{item.failure && FAILURE_LABELS[item.failure]}</span>
                <span className="flex gap-2">
                  {item.failure === 'network' && (
                    <Button
                      variant="ghost"
                      onClick={() => onRetry(item.id)}
                      className="text-media-scrim-fg hover:bg-media-scrim-bg active:bg-media-scrim-bg px-2 text-xs underline"
                    >
                      다시 시도
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    onClick={() => onDismiss(item.id)}
                    className="text-media-scrim-fg hover:bg-media-scrim-bg active:bg-media-scrim-bg px-2 text-xs underline"
                  >
                    지우기
                  </Button>
                </span>
              </Overlay>
            ) : (
              <Overlay>{STATUS_LABELS[item.status]}</Overlay>
            )}
          </Thumbnail>
        </li>
      ))}
    </ul>
  )
}

function Overlay({ children, role }: { children: ReactNode; role?: string }) {
  return (
    <span
      role={role}
      className="bg-media-scrim-bg/90 text-media-scrim-fg absolute inset-0 flex flex-col items-center justify-center gap-1 p-2 text-center text-xs leading-tight"
    >
      {children}
    </span>
  )
}
