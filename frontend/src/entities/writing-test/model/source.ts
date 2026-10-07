import type { WritingTestContext } from './types'

export interface WritingTestSource {
  name: string
  status: 'draft' | 'review' | 'finalized' | 'published'
  context: WritingTestContext
  attachments: { id: string; name: string; kind: 'photo' | 'video' }[]
}
export interface WritingTestSourceSummary {
  slug: string
  name: string
  updatedAt: string
  status: WritingTestSource['status']
}
