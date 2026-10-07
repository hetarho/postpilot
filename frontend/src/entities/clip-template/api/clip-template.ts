import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { invalidateGuidelines } from '@/entities/guideline/@x/clip-template'
import { CLIP_DEFAULT_REGION_PRESETS } from '@/entities/clip-design/@x/clip-template'
import { ClipTemplateService, type ProtoVideoTemplate } from '@/shared/api'
import { normalizeRecipe, type ClipTemplate, type ClipRecipe } from '../model/types'

export const clipTemplatesKey = (transport: Transport, ownerId: string) =>
  ['clip-templates', transport, ownerId] as const
export function toClipTemplate(value: ProtoVideoTemplate): ClipTemplate {
  return {
    id: value.id,
    name: value.name,
    compositionBody: value.compositionBody,
    // The server answers the selection a project would take; an older answer naming none is
    // the shared default (CLIP-166).
    introPreset: (value.introPreset ||
      CLIP_DEFAULT_REGION_PRESETS.intro) as ClipTemplate['introPreset'],
    outroPreset: (value.outroPreset ||
      CLIP_DEFAULT_REGION_PRESETS.outro) as ClipTemplate['outroPreset'],
    allowedCaptionStyles: [...value.allowedCaptionStyles],
    projectCount: value.projectCount,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  }
}
export function useClipTemplates(ownerId: string) {
  const transport = useTransport()
  const query = useQuery({
    queryKey: clipTemplatesKey(transport, ownerId),
    queryFn: async () =>
      (await createClient(ClipTemplateService, transport).listVideoTemplates({})).templates.map(
        toClipTemplate,
      ),
    enabled: !!ownerId,
    staleTime: 0,
    refetchOnMount: 'always',
  })
  return { ...query, templates: query.data ?? [] }
}
export function useClipTemplateMutations(ownerId: string) {
  const transport = useTransport()
  const client = createClient(ClipTemplateService, transport)
  const cache = useQueryClient()
  const invalidate = async () => {
    await Promise.all([
      cache.invalidateQueries({ queryKey: clipTemplatesKey(transport, ownerId) }),
      // Both clip list and detail share this owner-scoped family in T072.
      cache.invalidateQueries({ queryKey: ['clip-projects', transport, ownerId] }),
    ])
    // A 영상 지침's chips are this template's name, and a deleted one leaves it 적용 대상 없음.
    invalidateGuidelines(cache, transport, ownerId, 'clip')
  }
  const save = useMutation({
    mutationFn: async ({ id, recipe }: { id?: string; recipe: ClipRecipe }) => {
      const { name, compositionBody, introPreset, outroPreset, allowedCaptionStyles } =
        normalizeRecipe(recipe)
      // The docked 저장 sends the whole draft, the selection included; an empty style list is
      // a selection of none, which is why it travels in its wrapper (CLIP-142, CLIP-166).
      const fields = {
        name,
        compositionBody,
        introPreset,
        outroPreset,
        allowedCaptionStyles: { values: allowedCaptionStyles },
      }
      const response = id
        ? await client.updateVideoTemplate({ id, ...fields })
        : await client.createVideoTemplate(fields)
      if (!response.template?.id) throw new Error('Missing saved video template')
      return toClipTemplate(response.template)
    },
    onSuccess: invalidate,
  })
  const remove = useMutation({
    mutationFn: (id: string) => client.deleteVideoTemplate({ id }),
    onSuccess: invalidate,
  })
  return { save, remove }
}

/** Starting-design updates never reapply a captured name or composition. */
export function useClipTemplateDesignMutation(ownerId: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, recipe }: { id: string; recipe: ClipRecipe }) => {
      const result = await createClient(ClipTemplateService, transport).updateVideoTemplate({
        id,
        introPreset: recipe.introPreset,
        outroPreset: recipe.outroPreset,
        allowedCaptionStyles: { values: recipe.allowedCaptionStyles },
      })
      if (!result.template?.id) throw new Error('Missing saved video template')
      return toClipTemplate(result.template)
    },
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: clipTemplatesKey(transport, ownerId) })
      void cache.invalidateQueries({ queryKey: ['clip-projects', transport, ownerId] })
    },
  })
}
