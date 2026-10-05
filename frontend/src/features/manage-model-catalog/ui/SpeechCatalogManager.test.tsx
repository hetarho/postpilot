import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { AdminSpeechProfile, SpeechCandidate } from '@/entities/model-catalog'
import { ModelCatalogManager } from './ModelCatalogManager'
import { SpeechProfileForm } from './SpeechProfileForm'
import { i18n } from '../config/i18n'

const mocks = vi.hoisted(() => ({
  save: vi.fn(),
  refresh: vi.fn(),
  startQualification: vi.fn(),
  speech: {
    hasData: true,
    isPending: false,
    isError: false,
    fetchError: '',
    profiles: [] as AdminSpeechProfile[],
  },
}))
const models: SpeechCandidate[] = [
  {
    ref: { providerId: 'speech', modelId: 'design' },
    label: 'Explicit design',
    design: true,
    synthesis: false,
    korean: false,
    style: false,
    speakerBoost: false,
    requiresAlpha: false,
    maxText: 1000,
    tokenCostFactor: '',
    characterCostMultiplier: '',
    costDiscountMultiplier: '',
  },
  {
    ref: { providerId: 'speech', modelId: 'synth' },
    label: 'Korean synthesis',
    design: false,
    synthesis: true,
    korean: true,
    style: true,
    speakerBoost: true,
    requiresAlpha: false,
    maxText: 5000,
    tokenCostFactor: '1',
    characterCostMultiplier: '1',
    costDiscountMultiplier: '1',
  },
]

vi.mock('@/entities/model-catalog', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/model-catalog')>()),
  useAdminCatalog: () => ({
    catalog: { entries: [], fetchError: '', estimatorCombos: [] },
    isPending: false,
    isError: false,
  }),
  useRefreshCatalog: () => ({ refresh: vi.fn(), isPending: false }),
  useAdminSpeechProfiles: () => ({
    browse: {
      profiles: mocks.speech.profiles,
      choices: [],
      candidates: models,
      fetchError: mocks.speech.fetchError,
    },
    hasData: mocks.speech.hasData,
    isPending: mocks.speech.isPending,
    isError: mocks.speech.isError,
    refresh: mocks.refresh,
    refreshing: false,
    save: mocks.save,
    saving: false,
    startQualification: mocks.startQualification,
    qualifying: false,
  }),
}))
vi.mock('./CatalogDocumentPanel', () => ({ CatalogDocumentPanel: () => null }))

describe('the separate spoken voice admin tab', () => {
  beforeEach(() => {
    mocks.save.mockReset().mockResolvedValue(undefined)
    mocks.refresh.mockClear()
    mocks.startQualification.mockClear()
    Object.assign(mocks.speech, {
      hasData: true,
      isPending: false,
      isError: false,
      fetchError: '',
      profiles: [],
    })
  })

  it('does not claim no registrations when the saved list could not be read', async () => {
    mocks.speech.hasData = false
    mocks.speech.isError = true
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText(/조합 목록을 불러오지 못했습니다/)).toBeInTheDocument()
    expect(screen.queryByText(/등록한 목소리 모델 조합이 없습니다/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '목소리 모델 추가' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '목록 새로고침' })).toBeEnabled()
  })

  it.each([
    ['SPEECH_PROVIDER_NOT_CONFIGURED', /TTS 공급사 연결이 설정되지 않았습니다/],
    ['SPEECH_API_KEY_NOT_CONFIGURED', /TTS 공급사 API 키가 서버에 설정되지 않았습니다/],
    ['SPEECH_CATALOG_UNAVAILABLE', /TTS 공급사의 모델 목록을 읽지 못했습니다/],
  ])('explains %s and preserves access to saved profiles', async (reason, message) => {
    mocks.speech.fetchError = reason
    mocks.speech.profiles = [
      {
        id: 'saved',
        revision: 1n,
        label: 'Saved voice',
        grade: 'value',
        enabled: false,
        voiceReady: false,
        exportReady: false,
        prices: [],
        binding: {
          designModel: models[0].ref,
          speechModel: models[1].ref,
          settings: {
            stability: 0.5,
            similarityBoost: 0.75,
            style: 0,
            speakerBoost: false,
            speed: 1,
          },
          outputFormat: 'mp3_44100_128',
          descriptionMax: 1000,
          previewMax: 1000,
          speechMax: 1000,
        },
      },
    ]
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText(message)).toBeInTheDocument()
    expect(screen.getByText(/Saved voice/)).toBeInTheDocument()
    expect(screen.queryByText(/등록한 목소리 모델 조합이 없습니다/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '목소리 모델 추가' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: '조합 수정' }))
    expect(screen.getByLabelText('조합 이름')).toHaveValue('Saved voice')
    expect(mocks.save).not.toHaveBeenCalled()
    expect(mocks.startQualification).not.toHaveBeenCalled()
  })

  it('explains how to register a profile after a successful empty-list read', async () => {
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText(/생성·합성 모델과 요금 근거를 등록하세요/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '목소리 모델 추가' })).toBeEnabled()
  })
  it('opens without synthesis, selections or bulk-document writes and refreshes only speech metadata', async () => {
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText(/말투 모델과 별도로 저장/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '일괄 편집' })).not.toBeInTheDocument()
    expect(mocks.save).not.toHaveBeenCalled()
    expect(mocks.startQualification).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: '목록 새로고침' }))
    expect(mocks.refresh).toHaveBeenCalledOnce()
  })

  it('requires explicit model pair and grade, preserving decimal price evidence on save', async () => {
    const save = vi
      .fn<(profile: AdminSpeechProfile) => Promise<void>>()
      .mockResolvedValue(undefined)
    const user = userEvent.setup()
    render(
      <SpeechProfileForm
        profile={null}
        candidates={models}
        saving={false}
        onSave={save}
        onCancel={vi.fn()}
      />,
    )
    expect(screen.getByRole('button', { name: '조합 저장' })).toBeDisabled()
    await user.type(screen.getByLabelText('조합 이름'), 'Test voice')
    await user.click(screen.getByRole('combobox', { name: /목소리 생성 모델/ }))
    await user.click(screen.getByRole('option', { name: 'Explicit design' }))
    await user.click(screen.getByRole('combobox', { name: /더빙 합성 모델/ }))
    await user.click(screen.getByRole('option', { name: 'Korean synthesis' }))
    await user.click(screen.getByRole('combobox', { name: /등급/ }))
    await user.click(screen.getByRole('option', { name: '가성비' }))
    await user.click(screen.getAllByRole('button', { name: '요금 항목 추가' })[0])
    await user.type(screen.getAllByLabelText('요금 근거 URL')[0], 'https://example.com/pricing')
    await user.type(
      screen.getAllByLabelText('최대 청구량 근거 URL')[0],
      'https://example.com/bounds',
    )
    await user.type(screen.getAllByLabelText(/확인 시각/)[0], '2026-10-04T00:00:00Z')
    await user.click(
      screen.getAllByRole('checkbox', {
        name: '이 계정과 작업에 적용되는 요금 전체를 확인했습니다',
      })[0],
    )
    await user.click(screen.getByRole('combobox', { name: /청구 단위/ }))
    await user.click(screen.getByRole('option', { name: '공급자가 보고한 character-cost' }))
    await user.type(screen.getByLabelText('단위당 USD (소수 문자열)'), '0.000000001')
    await user.type(screen.getByLabelText('적용 배수'), '1.25')
    await user.type(screen.getByLabelText('요청 1회 최대 청구량'), '1000.5')
    await user.type(screen.getByLabelText('입력 글자당 최대 청구 단위 (상한 근거로 확인)'), '1')
    await user.click(screen.getByRole('button', { name: '조합 저장' }))
    await waitFor(() => expect(save).toHaveBeenCalledOnce())
    expect(save.mock.calls[0][0]).toMatchObject({
      revision: 0n,
      grade: 'value',
      binding: { designModel: models[0].ref, speechModel: models[1].ref, settings: { speed: 1 } },
      prices: [
        {
          operation: 'voice_design',
          charges: [
            {
              unit: 'character_cost',
              usdPerUnit: '0.000000001',
              multiplier: '1.25',
              maximumUnits: '1000.5',
              unitsPerInputCharacter: '1',
            },
          ],
        },
      ],
    })
  })
  it('provides matching operator vocabulary in Korean and English', () => {
    expect(Object.keys(i18n.ko.speechAdmin)).toEqual(Object.keys(i18n.en.speechAdmin))
    expect(Object.keys(i18n.ko.speechAdmin.operation)).toEqual(
      Object.keys(i18n.en.speechAdmin.operation),
    )
    expect(Object.keys(i18n.ko.speechAdmin.connectionReason)).toEqual(
      Object.keys(i18n.en.speechAdmin.connectionReason),
    )
    expect(Object.keys(i18n.ko.speechAdmin.units)).toEqual(Object.keys(i18n.en.speechAdmin.units))
  })
})
