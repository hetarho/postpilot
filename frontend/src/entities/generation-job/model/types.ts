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
  error: string
  postSlug: string
  observeModel: ModelRef | undefined
  writeModel: ModelRef | undefined
  createdAt: string
  updatedAt: string
}

export function isTerminal(job: Pick<GenerationJob, 'status'> | undefined): boolean {
  return job?.status === 'done' || job?.status === 'failed'
}

export function progressLabel(
  job: Pick<GenerationJob, 'stage' | 'progressDone' | 'progressTotal'>,
): string {
  switch (job.stage) {
    case 'observe':
      return `사진 ${job.progressDone}/${job.progressTotal} 관찰됨`
    case 'write':
      return '작성 중'
    case 'analyze':
      return '문체 분석 중'
    default:
      return '작업 준비 중'
  }
}
