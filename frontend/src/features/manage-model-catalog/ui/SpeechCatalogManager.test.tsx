import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { AdminSpeechProfile, SpeechCandidate } from '@/entities/model-catalog'
import { ModelCatalogManager } from './ModelCatalogManager'
import { SpeechProfileForm } from './SpeechProfileForm'
import { i18n } from '../config/i18n'

const mocks = vi.hoisted(() => ({ save: vi.fn(), refresh: vi.fn(), startQualification: vi.fn() }))
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
    browse: { profiles: [], choices: [], candidates: models, fetchError: '' },
    isPending: false,
    isError: false,
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
    expect(Object.keys(i18n.ko.speechAdmin.units)).toEqual(Object.keys(i18n.en.speechAdmin.units))
  })
})
