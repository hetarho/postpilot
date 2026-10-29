import { useStageSelection } from '@/entities/model-catalog'
import {
  WizardHeadingSuggestions,
  type WizardHeadingSuggestionsOperation,
} from '@/entities/template'

export { WizardHeadingChoices as HeadingChoices } from '@/entities/template'

export function HeadingSuggestions({
  operation,
  count,
}: {
  operation: WizardHeadingSuggestionsOperation
  count: number
}) {
  const write = useStageSelection('write')
  return (
    <WizardHeadingSuggestions
      operation={operation}
      count={count}
      writeModel={write.selected}
      modelPending={write.isPending}
    />
  )
}
