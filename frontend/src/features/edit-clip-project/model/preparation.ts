import type { ReactNode } from 'react'

export type ClipPreparationScreen = 'sources' | 'story' | 'style' | 'review'

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
  inputRefusal?: 'story' | 'style'
}
