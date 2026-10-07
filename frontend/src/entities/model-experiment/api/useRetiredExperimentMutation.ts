import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'
import { useCallback, useState } from 'react'
import { AppErrorDetailSchema, type AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

const retirementFailure: AppFailure = { reason: 'WRITING_TEST_LEGACY_READ_ONLY', params: {} }

/** Compatibility for callers that have not yet moved to the common writing test.
 * A legacy operation never reaches the transport, including retries and votes. */
export function useRetiredExperimentMutation() {
  const [error, setError] = useState<ConnectError | null>(null)
  const refuse = useCallback(async (): Promise<never> => {
    const refusal = new ConnectError(
      formatAppFailure(retirementFailure),
      Code.FailedPrecondition,
      undefined,
      [{ desc: AppErrorDetailSchema, value: create(AppErrorDetailSchema, retirementFailure) }],
    )
    setError(refusal)
    throw refusal
  }, [])
  return {
    refuse,
    isPending: false,
    isError: error !== null,
    error,
    failure: error ? retirementFailure : undefined,
    errorMessage: error ? formatAppFailure(retirementFailure) : '',
    reset: () => setError(null),
  }
}
