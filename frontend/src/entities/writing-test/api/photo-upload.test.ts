import { Code, ConnectError, createClient, createRouterTransport } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { UploadRejected, UploadRpcFailure } from '@/entities/image'
import { WritingTestOutput, WritingTestService as Service, WritingTestStage } from '@/shared/api'
import { createWritingTestClient } from './client'
import { planToProto } from './mappers'
import { observationPlan, testPlan } from './fixtures.test-support'
import { createTestPhotoHandshake } from './photo-upload'
import { createWritingTestStudioFixture } from './studio-fixture'

async function upload(
  fixture: ReturnType<typeof createWritingTestStudioFixture>,
  filename: string,
) {
  const handshake = createTestPhotoHandshake(fixture.transport)
  const presigned = await handshake.createUpload('', filename)
  return handshake.confirmUpload(presigned.uploadId, { width: 640, height: 480 })
}

describe('test photo handshake', () => {
  it('answers in the shapes of the post photo handshake', async () => {
    const fixture = createWritingTestStudioFixture()
    const handshake = createTestPhotoHandshake(fixture.transport)
    const presigned = await handshake.createUpload('ignored', 'one.jpg')
    expect(presigned).toEqual({
      uploadId: 'test-upload-1',
      putUrl: 'https://storage.test/put/test-upload-1',
      contentType: 'image/jpeg',
    })
    const confirmed = await handshake.confirmUpload(presigned.uploadId, { width: 640, height: 480 })
    expect(confirmed).toMatchObject({
      kind: 'photo',
      image: { id: 'test-photo-1', filename: 'one.jpg', width: 640, height: 480, bytes: 0 },
    })
    expect(Object.keys(presigned).sort()).toEqual(['contentType', 'putUrl', 'uploadId'])
    expect(fixture.calls).toEqual(['CreateTestPhotoUpload', 'ConfirmTestPhotoUpload'])
  })
  it('fails with the errors the upload batch classifies', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.createTestPhotoUpload, () => {
        throw new ConnectError('bad', Code.Unavailable)
      })
      rpc(Service.method.confirmTestPhotoUpload, () => {
        throw new ConnectError('down', Code.Unavailable)
      })
    })
    const handshake = createTestPhotoHandshake(transport)
    await expect(handshake.createUpload('', 'a.jpg')).rejects.toBeInstanceOf(UploadRpcFailure)
    await expect(handshake.createUpload('', 'a.jpg')).rejects.not.toBeInstanceOf(UploadRejected)
    await expect(handshake.confirmUpload('u', { width: 1, height: 1 })).rejects.toBeInstanceOf(
      UploadRpcFailure,
    )
  })
})

describe('studio fixture observation tests', () => {
  it('refuses what the server refuses', async () => {
    const fixture = createWritingTestStudioFixture()
    const client = createWritingTestClient(fixture.transport)
    const photo = await upload(fixture, 'one.jpg')
    if (photo.kind !== 'photo') throw new Error('expected a photo')
    const id = photo.image.id
    // A post test carrying photos is refused before it reaches the wire.
    const post = testPlan()
    post.context.photoIds = [id]
    await expect(client.estimate(post)).rejects.toThrow('photos')
    // The server's own shape rules hold even for a hand-built wire plan.
    const plan = observationPlan([id])
    await expect(client.estimate(plan)).resolves.toMatchObject({ credits: 6 })
    const unknown = observationPlan(['never-uploaded'])
    const quote = await client.estimate(unknown)
    await expect(
      client.start({ plan: unknown, requestKey: 'k', quoteKey: quote.quoteKey }),
    ).rejects.toMatchObject({ code: Code.InvalidArgument })
    expect(fixture.admissions).toHaveLength(1)
  })
  it('refuses a plan that breaks the observation shape on the wire', async () => {
    const fixture = createWritingTestStudioFixture()
    const raw = createClient(Service, fixture.transport)
    const wire = (change: (plan: ReturnType<typeof planToProto>) => void) => {
      const plan = planToProto(observationPlan(['p1']))
      change(plan)
      return raw.estimateWritingTest({ plan })
    }
    await expect(wire(() => {})).resolves.toBeDefined()
    await expect(wire((plan) => void (plan.context!.photoIds = []))).rejects.toMatchObject({
      code: Code.InvalidArgument,
    })
    await expect(
      wire((plan) => void (plan.context!.material!.text = 'note')),
    ).rejects.toMatchObject({ code: Code.InvalidArgument })
    await expect(
      wire((plan) => void (plan.modelStage = WritingTestStage.WRITE)),
    ).rejects.toMatchObject({ code: Code.InvalidArgument })
    await expect(wire((plan) => void (plan.output = WritingTestOutput.POST))).rejects.toMatchObject(
      { code: Code.InvalidArgument },
    )
  })
  it('echoes the started test with its photos and blind observations, naming observers on reveal', async () => {
    const fixture = createWritingTestStudioFixture()
    const client = createWritingTestClient(fixture.transport)
    const first = await upload(fixture, 'one.jpg')
    const second = await upload(fixture, 'two.jpg')
    if (first.kind !== 'photo' || second.kind !== 'photo') throw new Error('expected photos')
    const plan = observationPlan([first.image.id, second.image.id])
    const quote = await client.estimate(plan)
    const started = await client.start({ plan, requestKey: 'start', quoteKey: quote.quoteKey })
    expect(started.output).toBe('observation')
    expect(started.photos?.map((photo) => photo.filename)).toEqual(['one.jpg', 'two.jpg'])
    expect(started.candidates[0].observations).toHaveLength(2)
    expect(started.candidates[0].observations?.[0].model).toBeUndefined()
    const match = started.matches[0]
    const done = await client.decide({
      testId: started.id,
      expectedRevision: started.revision,
      requestKey: 'vote',
      matchId: match.id,
      winnerCandidateId: match.leftCandidateId,
    })
    expect(done.status).toBe('completed')
    expect(done.candidates[0].observations?.[0].model).toBe('model-0')
  })
})
