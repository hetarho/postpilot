import { Plus } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'
import { useWritingTestTranslation } from '@/features/writing-test'
import type { ComponentProps } from 'react'
import { ActionMenu } from '@/shared/ui'

/** 새 비교 (MODEL-44): one menu offering the two comparisons — 글 쓰는 AI opens a writing test on
 *  its candidates, 사진 보는 AI the photo observation test — each returning to `entry`. 지난 비교 and
 *  AI 토너먼트 mount this same menu; it moves through the router, never a reload. */
export function NewComparisonMenu({
  entry,
  triggerVariant = 'secondary',
}: {
  /** Where a comparison opened from this menu returns (THEME-58). */
  entry: string
  triggerVariant?: ComponentProps<typeof ActionMenu>['triggerVariant']
}) {
  const { t } = useWritingTestTranslation()
  const navigate = useNavigate()
  return (
    <ActionMenu
      label={t('newComparison.label')}
      triggerVariant={triggerVariant}
      triggerLabel={
        <>
          <Plus aria-hidden="true" className="size-5 shrink-0" />
          {t('newComparison.label')}
        </>
      }
      items={[
        {
          id: 'write',
          label: t('newComparison.write'),
          description: t('newComparison.writeHelp'),
          onSelect: () =>
            void navigate({
              to: '/tests',
              search: { factor: 'model', stage: 'write', count: 2, entry },
            }),
        },
        {
          id: 'observe',
          label: t('newComparison.observe'),
          description: t('newComparison.observeHelp'),
          onSelect: () => void navigate({ to: '/tests/photos', search: { count: 2, entry } }),
        },
      ]}
    />
  )
}
