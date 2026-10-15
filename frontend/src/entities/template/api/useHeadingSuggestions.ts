import { useMutation, useQuery } from '@connectrpc/connect-query'
import { TemplateService } from '@/shared/api'
import type { HeadingSuggestionInput } from '../model/heading-suggestions'

export function useStartHeadingSuggestions() {
  const mutation = useMutation(TemplateService.method.startTemplateHeadingSuggestions)
  return async (input: HeadingSuggestionInput) => {
    const result = await mutation.mutateAsync({
      ...input,
      targetPath: [...input.targetPath],
      sections: [...input.sections],
    })
    return result.jobId
  }
}

export function useHeadingSuggestionsEstimate(
  writeModel: HeadingSuggestionInput['writeModel'] | null,
) {
  const query = useQuery(
    TemplateService.method.estimateTemplateHeadingSuggestions,
    { writeModel: writeModel ?? undefined },
    { enabled: writeModel !== null },
  )
  return { free: query.data?.free ?? false, credits: query.data?.credits }
}

export function useHeadingSuggestionsResult(jobId: string, ready: boolean) {
  const query = useQuery(
    TemplateService.method.getTemplateHeadingSuggestions,
    { jobId },
    { enabled: ready && jobId !== '', staleTime: Infinity, retry: false },
  )
  return {
    groups: query.data?.sections.map((section) => [...section.headings]),
    error: query.error,
    refetch: query.refetch,
  }
}
