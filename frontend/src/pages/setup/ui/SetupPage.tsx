import { useSearch } from '@tanstack/react-router'
import { CreationSetup } from '@/widgets/creation-setup'
export function SetupPage() {
  const { restart } = useSearch({ from: '/authenticated/setup' })
  return <CreationSetup restart={restart} />
}
