import i18next from 'i18next'
import {
  appFailureFromProto,
  type AppFailure,
  contentLanguageFromProto,
  type ContentLanguage,
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
  failure: AppFailure | undefined
  platformPostUrl: string
  updatedAt: string
  targetLanguage: ContentLanguage | undefined
  contentLanguage: ContentLanguage | undefined
  voiceSourceLanguage: ContentLanguage | undefined
}

export const TERMINAL_PUBLISH_STATUSES = new Set([
  PublishStatus.PUBLISHED,
  PublishStatus.FAILED,
  PublishStatus.NEEDS_ATTENTION,
  PublishStatus.OUTCOME_UNKNOWN,
  PublishStatus.CANCELED,
])

export function publishStageLabel(stage: PublishStage): string {
  switch (stage) {
    case PublishStage.QUEUED:
      return i18next.t('stage.queued', { ns: 'publishing' })
    case PublishStage.CLAIMED:
      return i18next.t('stage.claimed', { ns: 'publishing' })
    case PublishStage.PREPARING:
      return i18next.t('stage.preparing', { ns: 'publishing' })
    case PublishStage.OPENING_EDITOR:
      return i18next.t('stage.openingEditor', { ns: 'publishing' })
    case PublishStage.FILLING_CONTENT:
      return i18next.t('stage.fillingContent', { ns: 'publishing' })
    case PublishStage.UPLOADING_PHOTOS:
      return i18next.t('stage.uploadingPhotos', { ns: 'publishing' })
    case PublishStage.COMMITTING:
      return i18next.t('stage.committing', { ns: 'publishing' })
    case PublishStage.VERIFYING:
      return i18next.t('stage.verifying', { ns: 'publishing' })
    case PublishStage.PUBLISHED:
      return i18next.t('stage.published', { ns: 'publishing' })
    default:
      return i18next.t('stage.progress', { ns: 'publishing' })
  }
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
    failure:
      job.failure ||
      job.status === PublishStatus.FAILED ||
      job.status === PublishStatus.NEEDS_ATTENTION ||
      job.status === PublishStatus.OUTCOME_UNKNOWN
        ? appFailureFromProto(job.failure)
        : undefined,
    platformPostUrl: job.platformPostUrl,
    updatedAt: job.updatedAt,
    targetLanguage: contentLanguageFromProto(job.targetLanguage),
    contentLanguage: contentLanguageFromProto(job.contentLanguage),
    voiceSourceLanguage: contentLanguageFromProto(job.voiceSourceLanguage),
  }
}
