import { createRouterTransport } from '@connectrpc/connect'
import { expect, it } from 'vitest'
import { ClipGenerationService } from '@/shared/api'
import { clipProjectCalls } from './calls'

// CLIP-235: a correction sends only the fields the owner changed, tells an emptied subject list
// apart from an untouched one, and reads back the observations marked by hand.
it('saves observation corrections and reads back the marked observations', async () => {
  const sent: {
    clipId: string
    edits: {
      observationId: string
      event?: string
      speech?: string
      subjects: string[]
      subjectsSet: boolean
    }[]
  }[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipGenerationService.method.saveClipObservationEdits, (request) => {
      sent.push(request)
      return {
        observations: {
          status: 'available',
          sources: [
            {
              ownerEdited: true,
              source: { id: 'a', fingerprint: 'f', filename: 'a.mp4', durationMs: 1000 },
              segments: [
                {
                  startMs: 0,
                  endMs: 1000,
                  event: '고친 장면',
                  quality: 'clear',
                  ownerEdited: true,
                },
              ],
            },
          ],
        },
      }
    })
  })
  const observations = await clipProjectCalls(transport).saveObservationEdits('clip', [
    { observationId: 'a/0', event: '고친 장면' },
    { observationId: 'a/1', subjects: [] },
  ])
  expect(sent[0]?.clipId).toBe('clip')
  expect(sent[0]?.edits[0]).toMatchObject({
    observationId: 'a/0',
    event: '고친 장면',
    subjects: [],
    subjectsSet: false,
  })
  expect(sent[0]?.edits[0]?.speech).toBeUndefined()
  expect(sent[0]?.edits[1]).toMatchObject({ observationId: 'a/1', subjects: [], subjectsSet: true })
  expect(observations.sources[0]?.ownerEdited).toBe(true)
  expect(observations.sources[0]?.segments[0]).toMatchObject({
    event: '고친 장면',
    ownerEdited: true,
  })
})
