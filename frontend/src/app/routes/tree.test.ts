import { expect, it } from 'vitest'
import { routeTree } from './router'

interface TreeNode {
  id: string
  fullPath?: string
  children?: TreeNode[]
}

function addresses(node: TreeNode): string[] {
  const own = node.fullPath ? [node.fullPath] : []
  return [...own, ...(node.children ?? []).flatMap(addresses)]
}

/** The whole address space, pinned. The tree is assembled from one file per group now (T268),
 *  and a group that forgets to export a route — or exports it under the wrong parent — changes
 *  a URL rather than failing to compile. This list is what refuses that. */
it('addresses exactly the product’s URLs, whatever file assembles them', () => {
  expect([...new Set(addresses(routeTree as unknown as TreeNode))].sort()).toEqual([
    '/',
    '/about',
    '/account',
    '/admin',
    '/admin/',
    '/admin/estimator',
    '/admin/models',
    '/ai-models',
    '/ai-models/experiments/$id',
    '/billing',
    '/billing/checkout',
    '/billing/method/fail',
    '/billing/method/success',
    '/clips',
    '/clips/$clipId',
    '/clips/new',
    '/forgot-password',
    '/guidelines',
    '/login',
    '/login/google/callback',
    '/plans',
    '/posts',
    '/posts/$slug',
    '/posts/new',
    '/publishing-agents',
    '/reset-password',
    '/signup',
    '/templates',
    '/templates/$templateId',
    '/templates/new',
    '/verify-email',
    '/video-templates',
    '/video-templates/$templateId',
    '/video-templates/new',
    '/voice',
    '/voice/$',
    '/voices',
    '/voices/$voiceId',
    '/voices/$voiceId/',
    '/voices/$voiceId/import',
    '/voices/$voiceId/rules',
    '/voices/$voiceId/rules/$id/compare',
    '/voices/$voiceId/validations',
    '/voices/$voiceId/validations/$id',
    '/voices/$voiceId/versions',
  ])
})
