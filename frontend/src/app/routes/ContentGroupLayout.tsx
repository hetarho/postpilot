import { Outlet } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { TabLinks, pageStyles } from '@/shared/ui'
import { CONTENT_GROUPS } from './navigation'

export function ContentGroupLayout({ group }: { group: keyof typeof CONTENT_GROUPS }) {
  const { t } = useTranslation('nav')
  return (
    <>
      <div className={pageStyles({ width: 'wide', className: 'py-4' })}>
        <TabLinks
          ariaLabel={t(group === 'writing' ? 'writingGroup' : 'videoGroup')}
          items={CONTENT_GROUPS[group].map(({ labelKey, to }) => ({
            to,
            label: t(labelKey),
            exact: false,
          }))}
        />
      </div>
      <Outlet />
    </>
  )
}
