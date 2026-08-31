export function TechnicalDetail({ label, detail }: { label: string; detail?: string }) {
  if (!detail) return null

  return (
    <details className="text-content-secondary mt-3 text-sm">
      <summary className="active:bg-row-bg-active min-h-11 cursor-pointer rounded-md px-4 py-3 select-none">
        {label}
      </summary>
      <pre className="bg-surface-recessed text-content-tertiary mt-2 rounded-md p-4 text-xs break-words whitespace-pre-wrap">
        {detail}
      </pre>
    </details>
  )
}
