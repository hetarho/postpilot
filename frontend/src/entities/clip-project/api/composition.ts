import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { ClipService, type ProtoClipProjectComposition } from '@/shared/api'
import type { ClipCompositionInputs, ClipProjectComposition } from '../model/composition'

export function toProjectComposition(
  value: ProtoClipProjectComposition | undefined,
): ClipProjectComposition | undefined {
  if (!value) return undefined
  if (value.snapshot?.version !== 1 || !value.snapshot.body || !value.inputs) {
    throw new Error('Invalid clip composition contract')
  }
  const { snapshot, inputs } = value
  return {
    snapshot: {
      version: snapshot.version,
      body: snapshot.body,
      templateId: snapshot.templateId,
      legacy: snapshot.legacy,
    },
    inputs: {
      values: { ...inputs.values },
      items: Object.fromEntries(
        Object.entries(inputs.items).map(([group, list]) => [
          group,
          list.items.map((item) => ({ id: item.id, values: { ...item.values } })),
        ]),
      ),
      associations: inputs.associations.map((a) => ({
        groupId: a.groupId,
        itemId: a.itemId,
        sourceId: a.sourceId,
        fingerprint: a.fingerprint,
        startMs: a.startMs,
        endMs: a.endMs,
      })),
    },
  }
}

export function compositionInputsToProto(inputs: ClipCompositionInputs) {
  return {
    values: { ...inputs.values },
    items: Object.fromEntries(
      Object.entries(inputs.items).map(([group, items]) => [
        group,
        { items: items.map((item) => ({ id: item.id, values: { ...item.values } })) },
      ]),
    ),
    associations: inputs.associations.map((a) => ({ ...a })),
  }
}

export function useClipCapabilities(ownerId: string) {
  const transport = useTransport()
  return useQuery({
    queryKey: ['clip-capabilities', transport, ownerId],
    enabled: !!ownerId,
    staleTime: 0,
    queryFn: async ({ signal }) => {
      const value = await createClient(ClipService, transport).getClipCapabilities({}, { signal })
      return {
        compositionVersion: value.compositionVersion,
        compositionPlanVersion: value.compositionPlanVersion,
      }
    },
  })
}
