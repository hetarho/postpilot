import {
  PublishStage,
  PublishStatus,
  type ProtoPublishJob,
  type PublishVisibility,
} from '@/shared/api'

export interface PublishJob {
  id: string
  postSlug: string
  agentId: string
  status: PublishStatus
  stage: PublishStage
  contentRevision: bigint
  categoryId: string
  visibility: PublishVisibility
  errorCode: string
  errorMessage: string
  platformPostUrl: string
  updatedAt: string
}

export const TERMINAL_PUBLISH_STATUSES = new Set([
  PublishStatus.PUBLISHED,
  PublishStatus.FAILED,
  PublishStatus.NEEDS_ATTENTION,
  PublishStatus.OUTCOME_UNKNOWN,
  PublishStatus.CANCELED,
])

export const PUBLISH_STAGE_LABELS: Record<number, string> = {
  [PublishStage.QUEUED]: 'Mac 연결을 기다리는 중',
  [PublishStage.CLAIMED]: 'Mac에서 작업을 받았어요',
  [PublishStage.PREPARING]: '발행 준비 중',
  [PublishStage.OPENING_EDITOR]: '네이버 편집기 여는 중',
  [PublishStage.FILLING_CONTENT]: '글 입력 중',
  [PublishStage.UPLOADING_PHOTOS]: '사진 올리는 중',
  [PublishStage.COMMITTING]: '네이버에 최종 발행 중',
  [PublishStage.VERIFYING]: '발행 결과 확인 중',
  [PublishStage.PUBLISHED]: '발행 완료',
}

export function toPublishJob(job: ProtoPublishJob): PublishJob {
  return {
    id: job.id,
    postSlug: job.postSlug,
    agentId: job.agentId,
    status: job.status,
    stage: job.stage,
    contentRevision: job.contentRevision,
    categoryId: job.categoryId,
    visibility: job.visibility,
    errorCode: job.errorCode,
    errorMessage: job.errorMessage,
    platformPostUrl: job.platformPostUrl,
    updatedAt: job.updatedAt,
  }
}
