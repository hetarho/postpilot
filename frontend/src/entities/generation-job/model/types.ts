import i18next from 'i18next'
import type { AppFailure, ContentLanguage } from '@/shared/api'

export interface ModelRef {
  providerId: string
  modelId: string
}

export interface GenerationJob {
  id: string
  kind: string
  status: string
  stage: string
  progressDone: number
  progressTotal: number
  failure: AppFailure | undefined
  postSlug: string
  observeModel: ModelRef | undefined
  writeModel: ModelRef | undefined
  createdAt: string
  updatedAt: string
  targetLanguage: ContentLanguage | undefined
}

export function isTerminal(job: Pick<GenerationJob, 'status'> | undefined): boolean {
  return job?.status === 'done' || job?.status === 'failed'
}

export function progressLabel(
  job: Pick<GenerationJob, 'stage' | 'progressDone' | 'progressTotal'>,
): string {
  switch (job.stage) {
    case 'observe':
      return i18next.t('generation.observingPhotos', {
        ns: 'posts',
        count: job.progressTotal,
        done: job.progressDone,
        total: job.progressTotal,
      })
    case 'write':
      return i18next.t('generation.writing', { ns: 'posts' })
    case 'analyze':
      return i18next.t('generation.analyzing', { ns: 'posts' })
    case 'compare_write':
      return i18next.t('generation.writeCandidates', {
        ns: 'posts',
        count: job.progressTotal,
        done: job.progressDone,
        total: job.progressTotal,
      })
    case 'compare_observe':
      return i18next.t('generation.observeCandidates', {
        ns: 'posts',
        count: job.progressTotal,
        done: job.progressDone,
        total: job.progressTotal,
      })
    case 'compare_analyze':
      return i18next.t('generation.analyzeCandidates', {
        ns: 'posts',
        count: job.progressTotal,
        done: job.progressDone,
        total: job.progressTotal,
      })
    default:
      return i18next.t('generation.preparing', { ns: 'posts' })
  }
}
