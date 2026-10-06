import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminSpeechProfile, SpeechCandidate } from '@/entities/model-catalog'
import { ModelCatalogManager } from './ModelCatalogManager'
import { SpeechCombinationRow } from './SpeechCombinationRow'
import { SpeechTariffEditor } from './SpeechTariffEditor'
import { i18n } from '../config/i18n'

const mocks = vi.hoisted(() => ({
  save: vi.fn(),
  saveTariff: vi.fn(),
  refresh: vi.fn(),
  startQualification: vi.fn(),
  speech: {
    hasData: true,
    isPending: false,
    isError: false,
    fetchError: '',
    profiles: [] as AdminSpeechProfile[],
    combinations: [] as AdminSpeechProfile[],
  },
}))
const models: SpeechCandidate[] = [
  {
    ref: { providerId: 'speech', modelId: 'design' },
    label: 'Design',
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
    label: 'Synth',
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
function profile(registered = false): AdminSpeechProfile {
  return {
    id: registered ? 'saved' : '',
    revision: registered ? 1n : 0n,
    label: 'Design → Synth',
    grade: '',
    enabled: registered,
    voiceReady: false,
    exportReady: false,
    prices: [],
    binding: {
      designModel: models[0].ref,
      speechModel: models[1].ref,
      settings: { stability: 0.5, similarityBoost: 0.75, style: 0, speakerBoost: false, speed: 1 },
      outputFormat: 'mp3_44100_128',
      descriptionMax: 1000,
      previewMax: 1000,
      speechMax: 1000,
    },
  }
}
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
      combinations: mocks.speech.combinations,
      tariff: null,
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
    saveTariff: mocks.saveTariff,
    savingTariff: false,
    startQualification: mocks.startQualification,
    qualifying: false,
  }),
}))
vi.mock('./CatalogDocumentPanel', () => ({ CatalogDocumentPanel: () => null }))

describe('list-based speech administration', () => {
  beforeEach(() => {
    mocks.save.mockReset().mockResolvedValue(undefined)
    mocks.saveTariff.mockReset().mockResolvedValue(undefined)
    mocks.refresh.mockClear()
    mocks.startQualification.mockClear()
    Object.assign(mocks.speech, {
      hasData: true,
      isPending: false,
      isError: false,
      fetchError: '',
      profiles: [],
      combinations: [profile()],
    })
  })
  it('registers by checking use without a form, pricing inputs or generation', async () => {
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText('Synth · Design')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '목소리 모델 추가' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('조합 이름')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('목소리 설명 최대 글자 수')).not.toBeInTheDocument()
    expect(
      screen.queryByLabelText('목소리 만들기 기본 단가 (1,000자당 USD)'),
    ).not.toBeInTheDocument()
    expect(mocks.save).not.toHaveBeenCalled()
    await user.click(screen.getByRole('checkbox', { name: 'Synth · Design 사용' }))
    expect(mocks.save).toHaveBeenCalledWith({
      profileId: '',
      expectedRevision: 0n,
      designModel: models[0].ref,
      speechModel: models[1].ref,
      enabled: true,
      grade: '',
    })
    expect(mocks.startQualification).not.toHaveBeenCalled()
  })
  it('deduplicates the candidate pair and saved registration and directly saves its grade', async () => {
    mocks.speech.profiles = [profile(true)]
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getAllByRole('checkbox', { name: 'Synth · Design 사용' })).toHaveLength(1)
    await user.click(screen.getByRole('combobox', { name: /등급/ }))
    await user.click(screen.getByRole('option', { name: '가성비' }))
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({ profileId: 'saved', expectedRevision: 1n, grade: 'value' }),
    )
    expect(screen.getByText(/더빙 대본 1000자까지/)).toBeInTheDocument()
    expect(screen.getByText(/요금 설정 또는 청구 단위 검증/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '목소리 생성 검증' })).toBeDisabled()
  })
  it.each([
    ['SPEECH_PROVIDER_NOT_CONFIGURED', /TTS 공급사 연결이 설정되지 않았습니다/],
    ['SPEECH_API_KEY_NOT_CONFIGURED', /TTS 공급사 API 키가 서버에 설정되지 않았습니다/],
    ['SPEECH_CATALOG_UNAVAILABLE', /TTS 공급사의 모델 목록을 읽지 못했습니다/],
  ])('explains %s and keeps saved registrations withdrawable', async (reason, message) => {
    mocks.speech.fetchError = reason
    mocks.speech.profiles = [profile(true)]
    mocks.speech.combinations = []
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText(message)).toBeInTheDocument()
    expect(screen.getByText('Synth · Design')).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: 'Synth · Design 사용' }))
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({ profileId: 'saved', enabled: false }),
    )
    await user.click(screen.getByRole('button', { name: '목록 새로고침' }))
    expect(mocks.refresh).toHaveBeenCalledOnce()
    expect(mocks.startQualification).not.toHaveBeenCalled()
  })
  it('does not claim an empty list on a failed saved-list read', async () => {
    mocks.speech.hasData = false
    mocks.speech.isError = true
    mocks.speech.combinations = []
    const user = userEvent.setup()
    render(<ModelCatalogManager />)
    await user.click(screen.getByRole('tab', { name: '목소리' }))
    expect(screen.getByText(/조합 목록을 불러오지 못했습니다/)).toBeInTheDocument()
    expect(screen.queryByText(/제공 가능한 목소리 모델 조합이 없습니다/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '목록 새로고침' })).toBeEnabled()
  })
  it('shows only supported optional sound controls and explains their scope', async () => {
    const user = userEvent.setup()
    const p = profile(true)
    render(
      <SpeechCombinationRow
        profile={p}
        styleSupported={false}
        metadataAvailable
        saving={false}
        onSave={mocks.save}
        onQualify={mocks.startQualification}
      />,
    )
    expect(screen.queryByLabelText('안정성 (0–1)')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '더빙 합성 설정' }))
    expect(screen.getByText(/목소리 후보 미리듣기에는 적용되지 않습니다/)).toBeInTheDocument()
    expect(screen.queryByLabelText('스타일 강도 (0–1)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('목소리 선명도 보정')).not.toBeInTheDocument()
    await user.clear(screen.getByLabelText('안정성 (0–1)'))
    await user.type(screen.getByLabelText('안정성 (0–1)'), '0.4')
    await user.click(screen.getByRole('button', { name: '합성 설정 저장' }))
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({ adjustments: { stability: 0.4, similarity: 0.75, style: 0 } }),
    )
  })
  it('prefills public references without verification and preserves exact shared prices', async () => {
    const user = userEvent.setup()
    render(
      <SpeechTariffEditor
        tariff={null}
        saving={false}
        unavailable={false}
        onSave={mocks.saveTariff}
      />,
    )
    expect(screen.getByLabelText('목소리 저장(확정) 1회 USD')).toHaveValue('0')
    expect(screen.getByLabelText('목소리 만들기 기본 단가 (1,000자당 USD)')).toHaveValue('0.08')
    expect(screen.getByLabelText('대본 읽기 기본 단가 (1,000자당 USD)')).toHaveValue('0.08')
    expect(screen.getByRole('button', { name: '공통 요금 저장' })).toBeDisabled()
    await user.clear(screen.getByLabelText('목소리 만들기 기본 단가 (1,000자당 USD)'))
    await user.type(screen.getByLabelText('목소리 만들기 기본 단가 (1,000자당 USD)'), '0.000001')
    await user.clear(screen.getByLabelText('대본 읽기 기본 단가 (1,000자당 USD)'))
    await user.type(screen.getByLabelText('대본 읽기 기본 단가 (1,000자당 USD)'), '0.100001')
    await user.click(screen.getByRole('checkbox', { name: /이 계정의 전체 요금/ }))
    await user.click(screen.getByRole('button', { name: '공통 요금 저장' }))
    await waitFor(() => expect(mocks.saveTariff).toHaveBeenCalledOnce())
    expect(mocks.saveTariff).toHaveBeenCalledWith(
      expect.objectContaining({
        revision: 0n,
        designUsdPerUnit: '0.000000001',
        speechUsdPerUnit: '0.000100001',
        confirmationUsd: '0',
        complete: true,
      }),
    )
  })
  it('requires fresh account evidence acknowledgement when reopening saved pricing', () => {
    render(
      <SpeechTariffEditor
        tariff={{
          revision: 4n,
          designUsdPerUnit: '0.0001',
          speechUsdPerUnit: '0.0001',
          confirmationUsd: '0',
          source: 'https://example.com/account',
          complete: true,
          checkedAt: '2026-10-06T00:00:00Z',
        }}
        saving={false}
        unavailable={false}
        onSave={mocks.saveTariff}
      />,
    )
    expect(screen.getByRole('checkbox', { name: /이 계정의 전체 요금/ })).not.toBeChecked()
    expect(screen.getByRole('button', { name: '공통 요금 저장' })).toBeDisabled()
  })
  it('explains the two model roles with friendly names and optional exact IDs', async () => {
    const user = userEvent.setup()
    const p = profile()
    p.label = 'eleven_multilingual_ttv_v2 → Eleven v4'
    p.binding.designModel.modelId = 'eleven_multilingual_ttv_v2'
    p.binding.speechModel.modelId = 'eleven_v4'
    render(
      <SpeechCombinationRow
        profile={p}
        styleSupported
        metadataAvailable
        saving={false}
        onSave={mocks.save}
        onQualify={mocks.startQualification}
      />,
    )
    expect(screen.getByText('Eleven v4 · Voice Design v2')).toBeInTheDocument()
    expect(screen.getByText('① 목소리 만들기: Voice Design v2')).toBeInTheDocument()
    expect(screen.getByText('② 대본 읽기: Eleven v4')).toBeInTheDocument()
    expect(screen.queryByText(/eleven_multilingual_ttv_v2/)).not.toBeInTheDocument()
    expect(screen.getByText(/목소리 후보 만들기 1회: 미리듣기 1000자 약/)).toBeInTheDocument()
    expect(screen.getByText(/대본 읽기: 대본 1000자 약/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '모델 ID 보기' }))
    expect(screen.getByText(/eleven_multilingual_ttv_v2/)).toBeInTheDocument()
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('fills the server’s unsaved empty tariff without replacing saved zero prices', () => {
    render(
      <SpeechTariffEditor
        tariff={{
          revision: 0n,
          designUsdPerUnit: '',
          speechUsdPerUnit: '',
          confirmationUsd: '',
          source: '',
          complete: false,
          checkedAt: '',
        }}
        saving={false}
        unavailable={false}
        onSave={mocks.saveTariff}
      />,
    )
    expect(screen.getByLabelText('목소리 만들기 기본 단가 (1,000자당 USD)')).toHaveValue('0.08')
    expect(screen.getByLabelText('대본 읽기 기본 단가 (1,000자당 USD)')).toHaveValue('0.08')
    expect(screen.getByLabelText('목소리 저장(확정) 1회 USD')).toHaveValue('0')
    expect(screen.getByRole('button', { name: '공통 요금 저장' })).toBeDisabled()
  })
  it('keeps Korean and English operator keys identical', () => {
    expect(Object.keys(i18n.ko.speechAdmin)).toEqual(Object.keys(i18n.en.speechAdmin))
  })
})
