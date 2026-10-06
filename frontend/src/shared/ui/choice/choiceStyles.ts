import { twMerge } from 'tailwind-merge'

/** Two peer destinations with room for a visible icon, name and explanation. */
export function choiceStyles(className?: string) {
  return twMerge(
    'bg-button-secondary-bg text-button-secondary-fg hover:bg-button-secondary-bg-hover active:bg-button-secondary-bg-active duration-base ease-standard group flex min-w-0 flex-col gap-6 rounded-xl p-6 text-left transition-colors sm:p-8',
    className,
  )
}
