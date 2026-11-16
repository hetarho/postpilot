import { useMemo } from 'react'
import { useStageSelection } from '@/entities/model-catalog'
import {
  savedHeadingLines,
  useTemplates,
  WizardHeadingSuggestions,
  type WizardHeadingSuggestionsOperation,
} from '@/entities/template'

export { WizardHeadingChoices as HeadingChoices } from '@/entities/template'

/** The post wizard's heading suggestions: the owner's saved templates lend their heading lines as
 *  an optional wording reference (TMPL-76). The directory is a read, so it starts no work. */
export function HeadingSuggestions({
  ownerId,
  operation,
  count,
}: {
  ownerId: string
  operation: WizardHeadingSuggestionsOperation
  count: number
}) {
  const write = useStageSelection('write')
  const { templates } = useTemplates(ownerId)
  const referenceHeadings = useMemo(() => savedHeadingLines(templates), [templates])
  return (
    <WizardHeadingSuggestions
      operation={operation}
      count={count}
      writeModel={write.selected}
      modelPending={write.isPending}
      referenceHeadings={referenceHeadings}
    />
  )
}
