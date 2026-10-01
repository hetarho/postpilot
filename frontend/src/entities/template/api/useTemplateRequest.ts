import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  TemplateService,
  appFailureFromConnect,
  contentLanguageToProto,
  type AppFailure,
  type ContentLanguage,
} from '@/shared/api'

/** A model as the request names it (MODEL-23: the client sends the write ref, the server infers
 *  none). Structural, so this entity does not reach into the model catalog's. */
export interface TemplateRequestModel {
  providerId: string
  modelId: string
}

/** A template's four authored texts, as a request sends and its answer returns them. The two
 *  generation numbers are not part of it: a request never sets them (TMPL-60). */
export interface TemplateDraftTexts {
  name: string
  description: string
  titleArea: string
  body: string
}

export interface TemplateRequestInput {
  writeModel: TemplateRequestModel
  language: ContentLanguage
  text: string
  draft: TemplateDraftTexts
  /** The stored template the request edits; absent on `/templates/new`. */
  templateId?: string
  /** A post attached as the sample (이 글 형식으로 템플릿 만들기, TMPL-64). */
  samplePostSlug?: string
}

export interface TemplateRequestResult {
  draft: TemplateDraftTexts
  /** Wishes about how to write, kept out of the template as material for 지침 (TMPL-61). */
  wishes: string[]
}

/** Starts a template request (TMPL-58). Its refusals — the box, the cap, the model, the sample,
 *  the plan and the balance — come back as a structured failure the box renders in place. */
export function useStartTemplateRequest(): {
  start: (input: TemplateRequestInput) => Promise<string>
  isPending: boolean
  failure: AppFailure | undefined
  reset: () => void
} {
  const mutation = useMutation(TemplateService.method.startTemplateRequest)
  return {
    start: async (input) => {
      const response = await mutation.mutateAsync({
        writeModel: input.writeModel,
        language: contentLanguageToProto(input.language),
        text: input.text,
        draft: input.draft,
        templateId: input.templateId,
        samplePostSlug: input.samplePostSlug,
      })
      return response.jobId
    },
    isPending: mutation.isPending,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    reset: mutation.reset,
  }
}

/** Stops a queued or running request (TMPL-63). A finished one answers OK and changes nothing. */
export function useCancelTemplateRequest(): { cancel: (jobId: string) => Promise<void> } {
  const mutation = useMutation(TemplateService.method.cancelTemplateRequest)
  return {
    cancel: async (jobId) => {
      await mutation.mutateAsync({ jobId })
    },
  }
}

/** A finished request's answer, read once its job is done. */
export function useTemplateRequestResult(
  jobId: string,
  ready: boolean,
): { result: TemplateRequestResult | undefined; failure: AppFailure | undefined } {
  const query = useQuery(
    TemplateService.method.getTemplateRequestResult,
    { jobId },
    { enabled: ready && jobId !== '', staleTime: Infinity, retry: false },
  )
  const draft = query.data?.draft
  return {
    result: query.data
      ? {
          draft: {
            name: draft?.name ?? '',
            description: draft?.description ?? '',
            titleArea: draft?.titleArea ?? '',
            body: draft?.body ?? '',
          },
          wishes: [...query.data.wishes],
        }
      : undefined,
    failure: query.error ? appFailureFromConnect(query.error) : undefined,
  }
}

/** What one request on the write model is expected to cost (QUOTA-67): 무료, about n credits, or
 *  nothing when no figure can be stated. Never a quote. */
export function useTemplateRequestEstimate(writeModel: TemplateRequestModel | null): {
  free: boolean
  credits: number | undefined
} {
  const query = useQuery(
    TemplateService.method.estimateTemplateRequest,
    { writeModel: writeModel ?? undefined },
    { enabled: writeModel !== null },
  )
  return { free: query.data?.free ?? false, credits: query.data?.credits }
}
