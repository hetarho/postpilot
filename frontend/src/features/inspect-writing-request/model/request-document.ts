import type { RequestInspectionView } from '@/entities/request-inspection'

export function issuedAtText(value: RequestInspectionView['issuedAt']): string | undefined {
  if (!value) return undefined
  const milliseconds = Number(value.seconds) * 1000 + value.nanos / 1_000_000
  const date = new Date(milliseconds)
  return Number.isFinite(date.getTime()) ? date.toISOString() : undefined
}

/** Copy the published application projection explicitly. Neither host prose, generated message
 * metadata nor unknown transport fields can become canonical writing or clipboard content. */
export function requestDocument(views: readonly RequestInspectionView[]): string {
  const sections: string[] = []
  for (const view of views) {
    const lines = [`Stage: ${view.stage}`, `State: ${view.status}`]
    if (view.status === 'unavailable') {
      lines.push('Request information unavailable')
      sections.push(lines.join('\n'))
      continue
    }
    const field = (name: string, value: string | number | bigint | boolean | undefined) =>
      lines.push(`${name}: ${value === undefined || value === '' ? 'Unknown' : String(value)}`)
    field('Mode', view.mode)
    field('Inspection format version', view.version)
    field('Prompt version', view.promptVersion)
    field('Schema version', view.schemaVersion)
    if (view.status === 'captured') {
      field('Call identity', view.callId)
      field('Issued at', issuedAtText(view.issuedAt))
    }
    const conditions = view.conditions
    field(
      'Model',
      conditions?.model ? `${conditions.model.providerId}/${conditions.model.modelId}` : undefined,
    )
    field('Maximum completion tokens', conditions?.maxCompletionTokens)
    field('Reasoning effort', conditions?.reasoningEffort)
    field('Structured output', conditions?.structuredOutput)
    field('Reasoning disabled', conditions?.disableReasoning)
    field('Reasoning option omitted', conditions?.reasoningOmitted)
    field('Default budget', conditions?.defaultBudget)
    field('Frozen execution conditions', conditions?.frozenExecution)
    const measures = view.measures
    field('Characters', measures?.characters)
    field('UTF-8 bytes', measures?.utf8Bytes)
    field('Reference token estimate', measures?.referenceTokenEstimate)
    field('Actual provider input tokens', measures?.providerPromptTokens)
    field('Actual provider output tokens', measures?.providerCompletionTokens)
    field('Actual provider reasoning tokens', measures?.providerReasoningTokens)
    for (const [index, fragment] of view.fragments.entries()) {
      lines.push('', `Fragment ${index + 1}: ${fragment.role}`)
      field('Material owner', fragment.authorship)
      field('Material role', fragment.materialRole)
      field('Fragment identity', fragment.id)
      field('Sources', fragment.sourceRefs.join(', '))
      field('Source files', fragment.sourceFiles?.join(', '))
      field('Activation', fragment.activation)
      lines.push(fragment.text)
    }
    for (const native of view.nativeFields ?? []) {
      lines.push('', `Additional field: ${native.id}`)
      field('Material owner', native.authorship)
      field('Material role', native.materialRole)
      field('Sources', native.sourceRefs.join(', '))
      field('Source files', native.sourceFiles.join(', '))
      field('Activation', native.activation)
      lines.push(native.text)
    }
    lines.push('', 'Applied code rules:', ...view.selectedRuleIds)
    lines.push('', 'Excluded rules and reasons:')
    for (const omission of view.omissions ?? []) {
      lines.push(`${omission.id}: ${omission.reason}`)
      field('Activation', omission.activation)
      field('Source files', omission.sourceFiles.join(', '))
    }
    for (const attachment of view.attachments ?? []) {
      lines.push(`Attachment: ${attachment.kind} · ${attachment.id}`)
    }
    lines.push('', 'Output contract:')
    field('Name', view.output?.name)
    field('Version', view.output?.version)
    if (view.output?.schema) lines.push(view.output.schema)
    field('Composer', view.composer)
    field('Parser', view.parser)
    field('Consumer', view.consumer)
    field('Activation', view.activation)
    field('Source files', view.sourceFiles?.join(', '))
    sections.push(lines.join('\n'))
  }
  return sections.join('\n\n---\n\n')
}
