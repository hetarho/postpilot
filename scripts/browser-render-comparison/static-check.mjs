import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'

const candidate = dirname(fileURLToPath(import.meta.url)), args = process.argv.slice(2)
const option = (name, fallback) => args.includes(name) ? args[args.indexOf(name) + 1] : fallback
const source = resolve(option('--source-root',resolve(candidate,'../..')))
const dependency = resolve(option('--dependency-root',source))
const ts = createRequire(dependency + '/frontend/package.json')('typescript')
const paths = {
  '@/*': [source + '/frontend/src/*'],
}
const names = ['contract.ts', 'entry.ts', 'audio-observer.ts', 'worker.ts']
const hashes = () => Object.fromEntries(names.map(name => [name,
  createHash('sha256').update(readFileSync(resolve(candidate, name))).digest('hex')]))
const adapterSha256 = hashes()
const head = () => execFileSync('git', ['rev-parse', 'HEAD'], { cwd: source, encoding: 'utf8' }).trim()
const sourceHeadStart = head()
const syntax = names.map(name => {
  const result = ts.transpileModule(readFileSync(resolve(candidate, name), 'utf8'), {
    fileName: name, reportDiagnostics: true,
    compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext, verbatimModuleSyntax: true },
  })
  return { name, diagnostics: (result.diagnostics ?? []).map(value => ts.flattenDiagnosticMessageText(value.messageText, ' ')) }
})
const options = {
  target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext, moduleResolution: ts.ModuleResolutionKind.Bundler,
  skipLibCheck: true, resolveJsonModule: true, jsx: ts.JsxEmit.ReactJSX, noEmit: true, types: [], paths,
  typeRoots: [dependency + '/frontend/node_modules/@types', dependency + '/node_modules/@types'],
  lib: ['lib.es2023.d.ts', 'lib.dom.d.ts', 'lib.dom.iterable.d.ts'],
}
const host = ts.createCompilerHost(options)
host.resolveModuleNames = (names, containingFile) => names.map(name => {
  const importer = name.startsWith('@/') || name.startsWith('.') || name.startsWith('/')
    ? containingFile : dependency + '/frontend/src/external-type-resolver.ts'
  return ts.resolveModuleName(name, importer, options, host).resolvedModule
})
const declarations = execFileSync('git', ['ls-files', 'frontend/src'], { cwd: source, encoding: 'utf8' })
  .trim().split('\n').filter(name => name.endsWith('.d.ts')).map(name => resolve(source, name))
const program = ts.createProgram([...names.map(name => resolve(candidate, name)), ...declarations, dependency + '/frontend/node_modules/vite/client.d.ts'], options, host)
const all = ts.getPreEmitDiagnostics(program), own = all.filter(value => value.file?.fileName.startsWith(candidate + '/'))
const diagnostic = value => ({
  file: value.file?.fileName, line: value.file ? value.file.getLineAndCharacterOfPosition(value.start ?? 0).line + 1 : null,
  message: ts.flattenDiagnosticMessageText(value.messageText, ' '),
})
console.log(JSON.stringify({
  scope: 'External adapter syntax and own-file semantic diagnostics only; no emit, browser/codec execution or required project CI claim',
  typescript: ts.version, syntax, ownDiagnosticCount: own.length,
  sourceRoot: source, sourceHeadStart, sourceHeadEnd: head(), adapterSha256,
  adapterUnchanged: JSON.stringify(adapterSha256) === JSON.stringify(hashes()),
  ownDiagnostics: own.map(diagnostic), importedSourceDiagnosticCount: all.length - own.length,
  importedSourceDiagnostics: all.filter(value => !own.includes(value)).map(diagnostic),
  actualExecution: false, qualification: false,
}, null, 2))
if (own.length || syntax.some(value => value.diagnostics.length)) process.exitCode = 1
