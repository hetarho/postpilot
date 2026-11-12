import type { ReactNode } from 'react'
import type { RequirementsCoverage } from '@/entities/advertiser-guide'

/** ①'s screens in the post guide's order (CLIP-225); `requirements` exists only while sponsored. */
export const CLIP_PREPARATION_SCREENS = [
  'kind',
  'requirements',
  'sources',
  'template',
  'content',
  'style',
  'note',
  'review',
] as const

export type ClipPreparationScreen = (typeof CLIP_PREPARATION_SCREENS)[number]

/** The screens a project's guide shows: every one but 요청 while 내 돈으로 다녀왔어요. */
export function clipPreparationScreens(campaign: boolean): readonly ClipPreparationScreen[] {
  return CLIP_PREPARATION_SCREENS.filter((screen) => campaign || screen !== 'requirements')
}

export interface ClipPreparationRow {
  id: string
  label: string
  value: ReactNode
  screen: Exclude<ClipPreparationScreen, 'review'>
}

/** The form owns its inputs and latest local draft; the widget composes sibling features. */
export interface ClipPreparationContent {
  screens: Record<Exclude<ClipPreparationScreen, 'review'>, ReactNode>
  rows: ClipPreparationRow[]
  actions: ReactNode
  refusal: ReactNode
  inputRefusal?: 'content' | 'style'
  /** 체험단으로 다녀왔어요: adds 요청 and the requirements tracker (CLIP-210, CLIP-228). */
  campaign: boolean
  /** The tracker's coverage of 필수 키워드 and the hashtags checked on ③, while sponsored. */
  requirements?: { coverage: RequirementsCoverage; hashtags: readonly string[] }
  /** 덧붙임's 다음 reads 건너뛰고 확인 until the instruction holds text (CLIP-225). */
  noteEmpty: boolean
}
