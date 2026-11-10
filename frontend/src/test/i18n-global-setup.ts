import type { TestProject } from 'vitest/node'
import type { I18nResources } from '@/app/providers/i18n/configure'
import { resources } from '@/app/providers/i18n/resources'

declare module 'vitest' {
  export interface ProvidedContext {
    /** The assembled i18n catalogs, evaluated once per run. Absent in watch mode. */
    i18nResources?: I18nResources
  }
}

/** Evaluates the i18n catalogs ONCE per run and hands them to every DOM test file as data.
 *
 *  The catalogs are ~120 modules, and an isolated test file evaluates its own copy of every
 *  module it imports: importing them from `setup.ts` cost each DOM file about 0.9 s of setup,
 *  more than most files spend on their tests. Each file receives its own structured clone of the
 *  value, so it still gets resource objects of its own to mutate, as a fresh import gave it.
 *
 *  Watch mode reruns files after a catalog edit without running this again, so a snapshot taken
 *  here would go stale there; each file then evaluates the catalogs itself, as before.
 *  `i18n-resources.test.ts` pins that what a file receives deep-equals the current catalogs. */
export default function setup(project: TestProject) {
  if (project.globalConfig.watch) return
  project.provide('i18nResources', resources)
}
