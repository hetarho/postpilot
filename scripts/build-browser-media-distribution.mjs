import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { createRequire } from 'node:module'
import { dirname, resolve, relative } from 'node:path'
import { pathToFileURL } from 'node:url'
import { execFileSync } from 'node:child_process'

const root=resolve(import.meta.dirname,'..')
const args=process.argv.slice(2)
const reportPath=resolve(args.includes('--report')?args[args.indexOf('--report')+1]:'tmp/browser-media-emitted-bundle.json')
const sha=bytes=>createHash('sha256').update(bytes).digest('hex')
const require=createRequire(resolve(root,'frontend/package.json'))
const {build}=await import(pathToFileURL(require.resolve('vite')).href)
const chunks=[]
function packageAt(id){
 const file=id.split('?')[0]
 if(!file.includes('/node_modules/')||!existsSync(file))return null
 for(let directory=dirname(file);directory!==dirname(directory);directory=dirname(directory)){
  const path=resolve(directory,'package.json')
  if(existsSync(path)){const p=JSON.parse(readFileSync(path,'utf8'));if(p.name&&p.version)return{name:p.name,version:p.version,declaredLicense:p.license??null,metadataSHA256:sha(readFileSync(path))}}
 }
 return null
}
function inventory(environment){return{name:'browser-media-emitted-'+environment,generateBundle(_options,bundle){
 for(const [file,item] of Object.entries(bundle))if(item.type==='chunk'){
  const modules=Object.keys(item.modules??{});const packages=new Map()
  for(const id of modules){const p=packageAt(id);if(p)packages.set(p.name+'@'+p.version,p)}
  chunks.push({environment,file,codeSHA256:sha(item.code),moduleCount:modules.length,packages:[...packages.values()].sort((a,b)=>a.name.localeCompare(b.name)),sourceModules:modules.filter(id=>id.startsWith(root+'/frontend/src/')).map(id=>relative(root,id)).sort()})
 }
}}}
const head=execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8'}).trim()
await build({root:resolve(root,'frontend'),configFile:resolve(root,'frontend/vite.config.ts'),plugins:[inventory('main')],worker:{plugins:()=>[inventory('worker')]}})
const mediaNames=new Set(['mediabunny','@resvg/resvg-wasm','@soundtouchjs/core','@soundtouchjs/interpolation-strategy-lanczos','pixi.js','pixi-filters'])
const mediaChunks=chunks.filter(c=>c.packages.some(p=>mediaNames.has(p.name)))
if(!mediaChunks.length)throw new Error('No emitted media module inventory was observed')
for(const c of mediaChunks){const actual=resolve(root,'frontend/dist',c.file);if(!existsSync(actual)||sha(readFileSync(actual))!==c.codeSHA256)throw new Error('Emitted chunk bytes changed: '+c.file)}
const record={version:1,sourceCommit:head,lockfileSHA256:sha(readFileSync(resolve(root,'pnpm-lock.yaml'))),node:process.version,chunks,mediaChunks:mediaChunks.map(c=>c.file),scope:'Actual Vite main and module Worker emitted JavaScript graph. Native dependencies inside the original WASM remain independently unverified; type-only installed packages are not inferred as emitted runtime code.',originalWasmDependencyGraphVerified:false,distributionApproved:false,qualification:false}
mkdirSync(dirname(reportPath),{recursive:true});writeFileSync(reportPath,JSON.stringify(record,null,2)+'\n')
console.log(JSON.stringify({mediaChunks:record.mediaChunks,chunks:chunks.length,report:reportPath,qualification:false}))
