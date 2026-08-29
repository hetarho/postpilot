import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceLearningService } from '@/shared/api'
import { voiceConfirmationsQueryKey } from './voice-queries'

export function useRuleConfirmations(ownerId: string) {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceConfirmationsQueryKey(transport, ownerId),
    queryFn: () => createClient(VoiceLearningService, transport).listRuleConfirmations({}),
    enabled: ownerId !== '',
  })
  return { confirmations: query.data?.confirmations ?? [], isPending: query.isPending }
}
