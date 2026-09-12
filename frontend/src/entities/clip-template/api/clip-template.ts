import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipService, type ProtoVideoTemplate } from '@/shared/api'
import {
  CLIP_ACCENTS,
  COPY_STYLES,
  normalizeRecipe,
  type ClipTemplate,
  type ClipRecipe,
  type CopyStyle,
  type ClipAccent,
} from '../model/types'

export const clipTemplatesKey = (transport: Transport, ownerId: string) =>
  ['clip-templates', transport, ownerId] as const
export function toClipTemplate(value: ProtoVideoTemplate): ClipTemplate {
  if (
    !['', 'steady', 'rapid'].includes(value.captionPace) ||
    value.copyStyles.some((s) => !COPY_STYLES.includes(s as CopyStyle)) ||
    !CLIP_ACCENTS.includes(value.accent as ClipAccent)
  )
    throw new Error('Invalid clip template contract')
  return {
    ...(value.compositionBody
      ? { compositionBody: value.compositionBody, compositionLegacy: value.compositionLegacy }
      : {}),
    ...(value.captionPace
      ? { captionPace: value.captionPace as NonNullable<ClipRecipe['captionPace']> }
      : {}),
    id: value.id,
    name: value.name,
    cutGuidance: value.cutGuidance,
    informationFields: value.informationFields.map((f) => ({ label: f.label, prompt: f.prompt })),
    copyStyles: value.copyStyles as CopyStyle[],
    accent: value.accent as ClipAccent,
    // Retained only for reading and converting legacy recipes.
    preset: value.preset as ClipRecipe['preset'],
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
      (await createClient(ClipService, transport).listVideoTemplates({})).templates.map(
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
  const client = createClient(ClipService, transport)
  const cache = useQueryClient()
  const invalidate = async () => {
    await Promise.all([
      cache.invalidateQueries({ queryKey: clipTemplatesKey(transport, ownerId) }),
      // Both clip list and detail share this owner-scoped family in T072.
      cache.invalidateQueries({ queryKey: ['clip-projects', transport, ownerId] }),
    ])
  }
  const save = useMutation({
    mutationFn: async ({ id, recipe }: { id?: string; recipe: ClipRecipe }) => {
      const { compositionBody, compositionLegacy, ...recipeFields } = normalizeRecipe(recipe)
      const fields = { ...recipeFields, captionPace: recipe.captionPace ?? 'steady' }
      const authored = compositionBody !== undefined && !compositionLegacy
      const response = authored
        ? id
          ? await client.updateVideoTemplate({ id, name: fields.name, compositionBody })
          : await client.createVideoTemplate({ name: fields.name, compositionBody })
        : id
          ? await client.updateVideoTemplate({
              id,
              ...fields,
              informationFields: { values: fields.informationFields },
              copyStyles: { values: fields.copyStyles },
            })
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
