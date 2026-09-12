import type { ClipObservations, ClipProject } from '@/entities/clip-project'
import { emptyClipProject } from '@/entities/clip-project'
import { clipEditingFixture } from './clip-editing'

export function clipObservationsFixture(): ClipObservations {
  const editing = clipEditingFixture()
  return {
    status: 'available',
    sources: [
      {
        source: editing.sources[0]!,
        segments: [
          {
            startMs: 3500,
            endMs: 10500,
            event: '접시에 담긴 음식을 가까이 촬영',
            subjects: ['음식', '접시'],
            speech: '맛있어요',
            quality: '선명하고 흔들림이 적음',
          },
          {
            startMs: 12000,
            endMs: 15000,
            event: '테이블을 비추는 장면',
            subjects: ['테이블'],
            speech: '',
            quality: '빠르게 흔들림',
          },
        ],
      },
      { source: editing.sources[1]!, segments: [] },
    ],
  }
}

export function observedClipFixture(): ClipProject {
  return {
    ...emptyClipProject(),
    id: 'project',
    title: '음식점 방문',
    videoTemplateId: 'template',
    disclosure: 'ad',
    createdAt: '2026-09-12T00:00:00Z',
    updatedAt: '2026-09-12T00:00:00Z',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: clipEditingFixture(),
    observations: clipObservationsFixture(),
    result: {
      contentType: 'video/mp4',
      bytes: 1000,
      durationMs: 19800,
      createdAt: '2026-09-12T00:00:00Z',
      viewUrl: 'https://example.test/clip.mp4',
      downloadUrl: 'https://example.test/download.mp4',
    },
  }
}
