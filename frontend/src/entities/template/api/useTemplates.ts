import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { TemplateService } from '@/shared/api'
import type { Template } from '../model/types'
import { templatesQueryKey, toTemplate } from './template-queries'

/** The one query behind the directory, shared by the management screen and every selector.
 *  A read and nothing else: mounting it creates no template and starts no job ([I5]).
 *
 *  `staleTime: 0` + `refetchOnMount: 'always'` against the app's 60s default, because
 *  `post_count` is a projection over POSTS: assigning a template in the editor changes it
 *  without touching any template, so no template mutation can invalidate it. A cached count is
 *  what the delete confirmation would otherwise state — the one number that must not be a
 *  guess, since the user confirms a detach against it. */
export function templateDirectoryQuery(transport: Transport, ownerId: string) {
  return {
    queryKey: templatesQueryKey(transport, ownerId),
    queryFn: () => createClient(TemplateService, transport).listTemplates({}),
    staleTime: 0,
    refetchOnMount: 'always' as const,
  }
}

export function useTemplates(ownerId: string): {
  templates: Template[]
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({ ...templateDirectoryQuery(transport, ownerId), enabled: ownerId !== '' })
  const templates = useMemo(() => query.data?.templates.map(toTemplate) ?? [], [query.data])
  return {
    templates,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
