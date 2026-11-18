import { ModelPicker } from '@/features/configure-model-pair'

/** A failed request's other model is chosen with the picker 홈's ▾ and AI 토너먼트's 지금 쓰는
 *  AI use: configuration authoring runs on the write stage's model. */
export function authoringModelChoice(onSaved: () => void) {
  return <ModelPicker stage="write" onSaved={onSaved} />
}
