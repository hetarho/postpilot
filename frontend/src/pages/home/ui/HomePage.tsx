import { useQuery } from '@connectrpc/connect-query'
import { HealthService } from '@/shared/api'

/** The scaffold's only screen. It renders the greeting and calls HealthService.Ping,
 *  which proves the whole wire end to end: browser → Connect → Go handler. Replace it
 *  with the real upload/compose flow. */
export function HomePage() {
  const { data, error, isPending } = useQuery(HealthService.method.ping, {})

  return (
    <main className="flex min-h-full flex-col items-center justify-center gap-4 bg-neutral-950 text-neutral-100">
      <h1 className="text-4xl font-semibold tracking-tight">Hello, world</h1>
      <p className="text-sm text-neutral-400">Postpilot</p>
      <p className="font-mono text-sm" data-testid="ping">
        {isPending && 'api: …'}
        {error && <span className="text-red-400">api: {error.message}</span>}
        {data && `api: ${data.message} (v${data.version})`}
      </p>
    </main>
  )
}
