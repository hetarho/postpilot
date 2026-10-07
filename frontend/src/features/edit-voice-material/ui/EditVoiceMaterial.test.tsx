import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useVoiceProfile, useVoiceSample, VoiceMaterialFreshness } from '@/entities/voice'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeVoiceOptions } from '@/test/voice'
import { EditVoiceMaterial } from './EditVoiceMaterial'
import { putMaterialPhoto } from '../api/photo'
vi.mock('../api/photo', () => ({
  prepareMaterialPhoto: vi.fn(async () => ({ blob: new Blob(['jpeg']), width: 1024, height: 768 })),
  putMaterialPhoto: vi.fn(async () => undefined),
}))
const writing = '직접 쓴 글이에요. '.repeat(40)
function host(options: FakeVoiceOptions = {}, sampleId = 'post') {
  const calls: string[] = [],
    onSaved = vi.fn(),
    onCancel = vi.fn()
  const updates: NonNullable<FakeVoiceOptions['updates']> = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    voice: {
      samples: [{ id: 'post', label: '내 글', body: writing }],
      analysis: {
        sourceVersionsKnown: true,
        acceptedSources: [{ sampleId: 'post', contentRevision: 1n }],
        ai: { impression: '저장된 이전 말투' },
      },
      updates,
      ...options,
    },
  })
  function Host() {
    const { profile } = useVoiceProfile('alice', 'voice-default')
    const { detail } = useVoiceSample('alice', 'voice-default', sampleId)
    return (
      <>
        {profile && (
          <>
            <p>{profile.analysis?.ai.impression}</p>
            <VoiceMaterialFreshness profile={profile} />
          </>
        )}
        {detail && (
          <EditVoiceMaterial
            ownerId="alice"
            voiceId="voice-default"
            detail={detail}
            prompt={
              detail.sample.kind === 'answer'
                ? { key: 'photo', part: 'description', photo: true, text: '사진 질문' }
                : undefined
            }
            onSaved={onSaved}
            onCancel={onCancel}
          />
        )}
      </>
    )
  }
  render(<Host />, { wrapper: withProviders(transport, createTestQueryClient()) })
  return { calls, updates, onSaved, onCancel }
}
describe('source editor transport host', () => {
  it('cancels an unsaved draft without source or AI mutations', async () => {
    const user = userEvent.setup(),
      { calls, onCancel } = host()
    await user.clear(await screen.findByLabelText('자료 제목'))
    await user.type(screen.getByLabelText('자료 제목'), '다른 제목')
    await user.click(screen.getByRole('button', { name: '취소' }))
    expect(onCancel).toHaveBeenCalledOnce()
    expect(calls.filter((call) => /^(Update|Analyze|Start|SetDefault)/.test(call))).toEqual([])
  })
  it('saves semantic source changes and keeps the accepted profile while marking reanalysis pending', async () => {
    const user = userEvent.setup(),
      { calls, updates, onSaved } = host()
    const field = await screen.findByLabelText('내가 쓴 글')
    await user.clear(field)
    await user.click(field)
    const changed = writing + '내용을 새로 고쳤어요.'
    await user.paste(changed)
    await user.dblClick(screen.getByRole('button', { name: '자료 저장' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
    expect(updates).toHaveLength(1)
    expect(updates[0]).toMatchObject({
      voiceId: 'voice-default',
      sampleId: 'post',
      expectedContentRevision: 1n,
      body: changed,
    })
    expect(updates[0]?.operationKey).toBeTruthy()
    expect(await screen.findByText('저장된 이전 말투')).toBeVisible()
    expect(
      await screen.findByText('학습 자료의 변경 내용이 아직 분석에 반영되지 않았어요.'),
    ).toBeVisible()
    expect(calls.filter((call) => /^(Analyze|Start|SetDefault|Estimate)/.test(call))).toEqual([])
  })
  it('confirms the exact same operation after the source save acknowledgement is lost', async () => {
    const user = userEvent.setup(),
      { updates, onSaved, calls } = host({ updateLosesFirstResponse: true })
    await user.clear(await screen.findByLabelText('자료 제목'))
    await user.type(screen.getByLabelText('자료 제목'), '수정된 제목')
    await user.click(screen.getByRole('button', { name: '자료 저장' }))
    await user.click(await screen.findByRole('button', { name: '같은 저장 다시 확인' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
    expect(updates).toHaveLength(2)
    expect(updates[0]).toEqual(updates[1])
    expect(calls.filter((call) => call === 'UpdateVoiceSample')).toHaveLength(2)
    expect(calls).not.toContain('AnalyzeVoice')
  })
  it('keeps a required photo when editing the answer and can explicitly replace it without AI', async () => {
    const user = userEvent.setup(),
      { updates, calls, onSaved } = host(
        {
          samples: [
            {
              id: 'photo-answer',
              label: '',
              kind: 'answer',
              promptKey: 'photo',
              body: '원래 답변',
              hasPhoto: true,
            },
          ],
          prompts: [{ key: 'photo', part: 'description', photo: true, text: '사진 질문' }],
          analysis: {
            sourceVersionsKnown: true,
            acceptedSources: [{ sampleId: 'photo-answer', contentRevision: 1n }],
            ai: { impression: '저장된 이전 말투' },
          },
        },
        'photo-answer',
      )
    expect(await screen.findByRole('img', { name: '답변에 사용한 사진' })).toHaveAttribute(
      'src',
      'https://storage.test/voices/photo-answer.jpg',
    )
    expect(screen.queryByRole('button', { name: '사진 삭제' })).toBeNull()
    await user.clear(screen.getByLabelText('저장한 답변'))
    await user.type(screen.getByLabelText('저장한 답변'), '고친 답변')
    await user.upload(
      screen.getByLabelText('사진 바꾸기', { selector: 'input' }),
      new File(['image'], 'photo.jpg', { type: 'image/jpeg' }),
    )
    await waitFor(() => expect(screen.getByRole('button', { name: '자료 저장' })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: '자료 저장' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
    expect(updates[0]).toMatchObject({ body: '고친 답변', photoUploadId: 'voice-upload-1' })
    expect(putMaterialPhoto).toHaveBeenCalledOnce()
    expect(calls.filter((call) => /^(Analyze|Start|SetDefault)/.test(call))).toEqual([])
  })
  it('omits photo replacement when only the saved answer text changes', async () => {
    const user = userEvent.setup(),
      { updates, calls, onSaved } = host(
        {
          samples: [
            {
              id: 'photo-answer',
              label: '',
              kind: 'answer',
              promptKey: 'photo',
              body: '원래 답변',
              hasPhoto: true,
            },
          ],
          prompts: [{ key: 'photo', part: 'description', photo: true, text: '사진 질문' }],
        },
        'photo-answer',
      )
    const answer = await screen.findByLabelText('저장한 답변')
    await user.clear(answer)
    await user.type(answer, '사진은 그대로 두고 고친 답변')
    await user.click(screen.getByRole('button', { name: '자료 저장' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
    expect(updates).toHaveLength(1)
    expect(updates[0]?.photoUploadId).toBeUndefined()
    expect(calls).not.toContain('CreateVoicePhotoUpload')
    expect(calls).not.toContain('AnalyzeVoice')
  })
})
