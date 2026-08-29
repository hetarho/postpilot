import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceLearningService, VoiceService, VoiceValidationService } from '@/shared/api'
import { toVoiceVersion, voiceConfirmationsQueryKey, voiceValidationsQueryKey, voiceVersionsQueryKey } from './voice-queries'

export function useVoiceDetails(ownerId: string) {
  const transport = useTransport()
  const versions = useQuery({ queryKey: voiceVersionsQueryKey(transport, ownerId), queryFn: () => createClient(VoiceService, transport).listVoiceProfileVersions({}), enabled: ownerId !== '' })
  const confirmations = useQuery({ queryKey: voiceConfirmationsQueryKey(transport, ownerId), queryFn: () => createClient(VoiceLearningService, transport).listRuleConfirmations({}), enabled: ownerId !== '' })
  const validations = useQuery({ queryKey: voiceValidationsQueryKey(transport, ownerId), queryFn: () => createClient(VoiceValidationService, transport).listVoiceProfileValidations({}), enabled: ownerId !== '' })
  return { versions: versions.data?.versions.map(toVoiceVersion) ?? [], confirmations: confirmations.data?.confirmations ?? [], validations: validations.data?.validations ?? [], isPending: versions.isPending || confirmations.isPending || validations.isPending }
}
