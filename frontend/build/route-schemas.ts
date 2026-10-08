import { readFileSync } from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import type { Plugin } from 'vite'

interface Reexport {
  local: ts.ModuleExportName
  from: string
}

function pageIndexExports(pages: string, page: string) {
  const filename = path.resolve(pages, page, 'index.ts')
  const source = ts.createSourceFile(
    filename,
    readFileSync(filename, 'utf8'),
    ts.ScriptTarget.Latest,
  )
  const names = new Map<string, Reexport>()
  for (const statement of source.statements) {
    if (
      !ts.isExportDeclaration(statement) ||
      statement.isTypeOnly ||
      statement.attributes ||
      !statement.moduleSpecifier ||
      !ts.isStringLiteral(statement.moduleSpecifier) ||
      !statement.exportClause ||
      !ts.isNamedExports(statement.exportClause)
    )
      continue
    for (const spec of statement.exportClause.elements) {
      if (!spec.isTypeOnly)
        names.set(spec.name.text, {
          local: spec.propertyName ?? spec.name,
          from: statement.moduleSpecifier.text,
        })
    }
  }
  return names
}

function importBinding(imported: ts.ModuleExportName | undefined, local: ts.Identifier) {
  const name =
    imported &&
    (ts.isStringLiteral(imported)
      ? ts.factory.createStringLiteral(imported.text, true)
      : ts.factory.createIdentifier(imported.text))
  return ts.factory.createImportSpecifier(false, name, ts.factory.createIdentifier(local.text))
}

/** Model-only route imports bypass side-effect-free page barrels in builds; UI remains lazy. */
export function routeSchemasPastPageIndex(pages: string) {
  const printer = ts.createPrinter({ omitTrailingSemicolon: true, removeComments: true })
  return {
    name: 'route-schemas-past-page-index',
    apply: 'build',
    enforce: 'pre',
    transform(code: string, id: string) {
      if (!/\/src\/app\/routes\/[^/]+\.ts$/.test(id.split('?')[0]!)) return null
      const source = ts.createSourceFile(id, code, ts.ScriptTarget.Latest)
      const edits: { start: number; end: number; text: string }[] = []
      const emit = (module: string, bindings: ts.ImportSpecifier[], typeOnly = false) =>
        printer.printNode(
          ts.EmitHint.Unspecified,
          ts.factory.createImportDeclaration(
            undefined,
            ts.factory.createImportClause(
              typeOnly ? ts.SyntaxKind.TypeKeyword : undefined,
              undefined,
              ts.factory.createNamedImports(bindings),
            ),
            ts.factory.createStringLiteral(module, true),
          ),
          source,
        )
      for (const statement of source.statements) {
        if (
          !ts.isImportDeclaration(statement) ||
          statement.attributes ||
          !ts.isStringLiteral(statement.moduleSpecifier)
        )
          continue
        const clause = statement.importClause
        if (
          !clause ||
          clause.name ||
          clause.phaseModifier ||
          !clause.namedBindings ||
          !ts.isNamedImports(clause.namedBindings) ||
          !clause.namedBindings.elements.length
        )
          continue
        const module = statement.moduleSpecifier.text
        if (!module.startsWith('@/pages/')) continue
        const page = module.slice('@/pages/'.length)
        if (!/^[a-z-]+$/.test(page)) continue
        const values = clause.namedBindings.elements.filter((spec) => !spec.isTypeOnly)
        const exported = values.length ? pageIndexExports(pages, page) : undefined
        const runtime = values.map((spec) => ({
          spec,
          target: exported?.get((spec.propertyName ?? spec.name).text),
        }))
        if (runtime.some(({ target }) => !target?.from.startsWith('./model/'))) continue
        const rewritten = runtime.map(({ spec, target }) =>
          emit(`@/pages/${page}/${target!.from.slice(2)}`, [
            importBinding(target!.local, spec.name),
          ]),
        )
        const types = clause.namedBindings.elements.filter((spec) => spec.isTypeOnly)
        if (types.length)
          rewritten.push(
            emit(
              module,
              types.map((spec) => importBinding(spec.propertyName, spec.name)),
              true,
            ),
          )
        edits.push({
          start: statement.getStart(source),
          end: statement.end,
          text: rewritten.join('\n'),
        })
      }
      for (const edit of edits.reverse())
        code = code.slice(0, edit.start) + edit.text + code.slice(edit.end)
      return code
    },
  } satisfies Plugin
}
