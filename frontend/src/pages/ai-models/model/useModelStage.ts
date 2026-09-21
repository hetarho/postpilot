import { useSearch } from '@tanstack/react-router'

export function useModelStage() {
  const { stage } = useSearch({ from: '/authenticated/models' })
  return { stage: stage ?? 'observe' }
}
