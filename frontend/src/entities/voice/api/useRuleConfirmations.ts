import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceLearningService } from '@/shared/api'
import { voiceConfirmationsQueryKey } from './voice-queries'

export function useRuleConfirmations(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceConfirmationsQueryKey(transport, ownerId, voiceId),
    queryFn: () => createClient(VoiceLearningService, transport).listRuleConfirmations({ voiceId }),
    enabled: ownerId !== '' && voiceId !== '',
  })
  return { confirmations: query.data?.confirmations ?? [], isPending: query.isPending }
}
