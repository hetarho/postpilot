import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceValidationService } from '@/shared/api'
import { voiceValidationsQueryKey } from './voice-queries'

export function useVoiceValidations(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceValidationsQueryKey(transport, ownerId, voiceId),
    queryFn: () =>
      createClient(VoiceValidationService, transport).listVoiceProfileValidations({ voiceId }),
    enabled: ownerId !== '' && voiceId !== '',
  })
  return { validations: query.data?.validations ?? [], isPending: query.isPending }
}
