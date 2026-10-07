import { createServer } from 'node:https'
import { createReadStream, existsSync, readFileSync, writeFileSync, mkdirSync, readdirSync, statSync, realpathSync } from 'node:fs'
import { createHash, randomBytes } from 'node:crypto'
import { createRequire } from 'node:module'
import { resolve, dirname, relative, extname } from 'node:path'
import { pathToFileURL } from 'node:url'
import { execFileSync } from 'node:child_process'

const args=process.argv.slice(2)
const option=(name,fallback)=>args.includes(name)?args[args.indexOf(name)+1]:fallback
const source=resolve(option('--source',process.cwd()))
const artifact=resolve(option('--artifacts',resolve(source,'frontend/dist')))
const prepared=option('--prepared',null)
const configPath=name=>prepared?resolve(prepared,'prepared-config',name):resolve(source,name==='_headers'?'frontend/public/_headers':'deploy/r2-cors.json')
const output=resolve(option('--output',resolve(source,'tmp/browser-static-deployment')))
const fixturePath=resolve(option('--fixture',resolve(source,'tmp/browser-media-fixtures/original-cfr60.mp4')))
if(!existsSync(fixturePath))throw new Error('Supply --fixture with an authorized synthetic CFR MP4.')
mkdirSync(output,{recursive:true,mode:0o700})
const hash=bytes=>createHash('sha256').update(bytes).digest('hex')
const readJSON=path=>JSON.parse(readFileSync(path,'utf8'))
const revision=execFileSync('git',['rev-parse','HEAD'],{cwd:source,encoding:'utf8'}).trim()
const require=createRequire(resolve(source,'package.json'))
const frontendRequire=createRequire(resolve(source,'frontend/package.json'))
const {chromium}=require('playwright')
const {build}=await import(pathToFileURL(frontendRequire.resolve('vite')).href)
const sourceFile=resolve(source,'frontend/src/shared/lib/media/video/range-source.ts')
const diagnosticModules=[]
await build({configFile:false,root:resolve(source,'frontend'),logLevel:'warn',publicDir:false,
  build:{outDir:resolve(output,'range-library'),emptyOutDir:true,minify:false,lib:{entry:sourceFile,formats:['es'],fileName:'range-adapter'}},
  plugins:[{name:'record-diagnostic-inputs',generateBundle(){for(const id of this.getModuleIds())if(existsSync(id)&&statSync(id).isFile())diagnosticModules.push({file:relative(source,id),sha256:hash(readFileSync(id))})}}]})
const certDir=resolve(output,'private-tls');mkdirSync(certDir,{recursive:true,mode:0o700})
const keyPath=resolve(certDir,'key.pem'),certPath=resolve(certDir,'cert.pem')
execFileSync(option('--openssl','openssl'),['req','-x509','-newkey','rsa:2048','-nodes','-days','2','-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1','-keyout',keyPath,'-out',certPath],{stdio:'ignore'})
execFileSync('chmod',['600',keyPath,certPath])
const tls={key:readFileSync(keyPath),cert:readFileSync(certPath)}
const cors=readJSON(configPath('r2-cors.json'))
const configuredOrigin=option('--configured-origin','https://postpilot.<domain>')
function policyIssues(policy){
 const rules=policy.rules??[],issues=[]
 if(rules.some(r=>r.allowed?.origins?.includes('*')))issues.push('wildcard_origin')
 const scoped=rules.filter(r=>r.allowed?.origins?.includes(configuredOrigin))
 if(!scoped.length)issues.push('configured_origin_missing')
 if(!scoped.some(r=>['GET','PUT','HEAD'].every(m=>r.allowed.methods?.includes(m))))issues.push('required_method_missing')
 if(!scoped.some(r=>['range','content-type','if-none-match'].every(h=>r.allowed.headers?.map(x=>x.toLowerCase()).includes(h))))issues.push('required_header_missing')
 if(!scoped.some(r=>['etag','content-range'].every(h=>r.exposeHeaders?.map(x=>x.toLowerCase()).includes(h))))issues.push('required_exposed_header_missing')
 return issues
}
const policyVariants={correct:cors,wildcard:structuredClone(cors),foreignOnly:structuredClone(cors),missingExpose:structuredClone(cors),missingGet:structuredClone(cors),missingRangeHeader:structuredClone(cors)}
policyVariants.wildcard.rules.forEach(r=>r.allowed.origins=['*'])
policyVariants.foreignOnly.rules.forEach(r=>r.allowed.origins=['https://foreign.invalid'])
policyVariants.missingExpose.rules.forEach(r=>r.exposeHeaders=r.exposeHeaders.filter(h=>h.toLowerCase()!=='content-range'))
policyVariants.missingGet.rules.forEach(r=>r.allowed.methods=r.allowed.methods.filter(m=>m!=='GET'))
policyVariants.missingRangeHeader.rules.forEach(r=>r.allowed.headers=r.allowed.headers.filter(h=>h.toLowerCase()!=='range'))
if(policyIssues(cors).length)throw new Error('Configured private CORS policy invalid: '+policyIssues(cors).join(','))
const headersText=readFileSync(configPath('_headers'),'utf8')
const fontManifest=readJSON(resolve(source,'frontend/src/entities/clip-design/config/clip-ink-fonts.json'))
const ink=readJSON(resolve(source,'frontend/src/entities/clip-design/config/clip-ink-identity.json'))
const fixture=readFileSync(fixturePath)
const token=randomBytes(16).toString('hex')
const notices=prepared?resolve(prepared,'prepared-public/licenses/browser-media'):resolve(source,'frontend/public/licenses/browser-media')
const requests=[],staticBodies=[]
let appOrigin
function headerRules(path){
  let match=false;const result={}
  for(const line of headersText.split('\n')){
    if(!line.trim()||line.trimStart().startsWith('#'))continue
    if(!/^\s/.test(line)){
      const pattern=line.replace(/[.+?^${}()|[\]\\]/g,'\\$&').replace('*','.*')
      match=new RegExp('^'+pattern+'$').test(path)
    }else if(match){const at=line.indexOf(':');result[line.slice(0,at).trim()]=line.slice(at+1).trim()}
  }
  return result
}
function mime(path){return ({'.js':'text/javascript','.mjs':'text/javascript','.wasm':'application/wasm','.html':'text/html; charset=utf-8','.json':'application/json','.ttf':'font/ttf','.txt':'text/plain; charset=utf-8'})[extname(path)]??'application/octet-stream'}
function fileAt(root,path){
  const candidate=resolve(root,'.'+path)
  if(!candidate.startsWith(root+'/')||!existsSync(candidate)||!statSync(candidate).isFile())return
  if(!realpathSync(candidate).startsWith(realpathSync(root)+'/'))return
  return candidate
}
const app=createServer(tls,(req,res)=>{
  const path=new URL(req.url,'https://localhost').pathname
  if(path==='/__probe__/'){
    res.writeHead(200,{'Content-Type':'text/html'});res.end('<!doctype html><meta charset=utf-8><title>Private static deployment probe</title><script type=module>import {createFiniteMediaSource} from "/__probe__/range-adapter.js"; window.rangeProbe=async(url)=>{const x=createFiniteMediaSource({kind:"url",url},{maxFileBytes:8388608,maxReadBytes:65536,maxReadTotalBytes:131072,maxCacheBytes:65536,timeoutMs:5000},new AbortController().signal);try{const size=await x.getSize();const bytes=await x.read(0,32);return{size,bytes:Array.from(bytes),measurements:x.measurements()}}catch(e){return{error:e.code??e.name}}finally{x.dispose()}}</script>');return
  }
  let file
  if(path==='/__probe__/range-adapter.js')file=resolve(output,'range-library/range-adapter.js')
  else if(path.startsWith('/licenses/browser-media/'))file=fileAt(notices,path.slice('/licenses/browser-media'.length))
  else file=fileAt(artifact,path)
  if(!file){res.writeHead(404);res.end();return}
  const bytes=readFileSync(file);staticBodies.push({path,bytes:bytes.length,sha256:hash(bytes)})
  res.writeHead(200,{'Content-Type':mime(file),'Content-Length':bytes.length,...headerRules(path)})
  res.end(bytes)
})
await new Promise(resolve=>app.listen(0,'127.0.0.1',resolve))
appOrigin='https://127.0.0.1:'+app.address().port
const storage=createServer(tls,(req,res)=>{
  const url=new URL(req.url,'https://localhost'),mode=url.searchParams.get('mode')??'correct'
  const origin=req.headers.origin
  if(url.searchParams.get('cap')!==token){res.writeHead(403);res.end();return}
  const policy=policyVariants[url.searchParams.get('policy')??'correct']
  if(!policy){res.writeHead(400);res.end();return}
  const mappedOrigin=origin===appOrigin?configuredOrigin:origin
  const requestedMethod=req.method==='OPTIONS'?req.headers['access-control-request-method']:req.method
  const rule=policy.rules.find(r=>(r.allowed.origins.includes(mappedOrigin)||r.allowed.origins.includes('*'))&&r.allowed.methods.includes(requestedMethod))
  const requestedHeaders=String(req.headers['access-control-request-headers']??'').split(',').map(h=>h.trim().toLowerCase()).filter(Boolean)
  const allowed=!!rule&&(req.method!=='OPTIONS'||requestedHeaders.every(h=>rule.allowed.headers.map(x=>x.toLowerCase()).includes(h)))
  if(allowed){res.setHeader('Access-Control-Allow-Origin',rule.allowed.origins.includes('*')?'*':origin);res.setHeader('Vary','Origin');res.setHeader('Access-Control-Expose-Headers',mode==='hidden'?'etag':rule.exposeHeaders.join(','))}
  if(req.method==='OPTIONS'){
    if(allowed){res.setHeader('Access-Control-Allow-Methods',rule.allowed.methods.join(','));res.setHeader('Access-Control-Allow-Headers',rule.allowed.headers.join(','))}
    res.writeHead(allowed?204:403);res.end();return
  }
  const m=req.headers.range?.match(/^bytes=(\d+)-(\d+)$/)
  const start=m?Number(m[1]):0,end=m?Number(m[2]):fixture.length-1
  requests.push({mode,method:req.method,originAllowed:allowed,rangeHeader:req.headers.range??null,start,end,totalBytes:fixture.length})
  if(!m||start<0||end>=fixture.length||end<start){res.writeHead(416);res.end();return}
  if(mode==='whole'){
    res.writeHead(200,{'Content-Type':'video/mp4','Content-Length':fixture.length});res.end(fixture);return
  }
  res.writeHead(206,{'Content-Type':'video/mp4','Content-Length':end-start+1,'Content-Range':`bytes ${mode==='wrong-offset'?start+1:start}-${end}/${fixture.length}`,'ETag':'"owned-synthetic"'})
  res.end(fixture.subarray(start,end+1))
})
await new Promise(resolve=>storage.listen(0,'127.0.0.1',resolve))
const storageOrigin='https://127.0.0.1:'+storage.address().port
const browser=await chromium.launch({executablePath:option('--browser','/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'),args:['--host-resolver-rules=MAP localhost 127.0.0.1']})
const report={version:1,sourceRevision:revision,sourceRangeSHA256:hash(readFileSync(sourceFile)),builtArtifactRoot:artifact,diagnosticBundleModules:diagnosticModules,preparedCORS_SHA256:hash(readFileSync(configPath('r2-cors.json'))),preparedHeadersSHA256:hash(Buffer.from(headersText)),syntheticOnly:true,qualified:false,distributionApproved:false,providerCalls:0,liveDeploymentExecuted:false,publicDomainTLSVerified:false,headersAppliedBy:'Isolated loopback static server interprets prepared _headers; actual Cloudflare deployment remains unexecuted.',privateFixtureDigest:hash(fixture),checks:[],passed:false}
const save=()=>writeFileSync(resolve(output,'report.json'),JSON.stringify(report,null,2)+'\n')
save()
try{
 const page=await browser.newPage({ignoreHTTPSErrors:true})
 await page.goto(appOrigin+'/__probe__/');await page.waitForFunction(()=>typeof window.rangeProbe==='function')
 report.browser=await browser.version();report.node=process.version
 report.secureContext=await page.evaluate(()=>({secureContext:isSecureContext,opfs:typeof navigator.storage.getDirectory==='function',videoEncoder:typeof VideoEncoder==='function',offscreenCanvas:typeof OffscreenCanvas==='function'}))
 if(!report.secureContext.secureContext)throw new Error('HTTPS did not establish secure context')
 for(const mode of ['correct','hidden','wrong-offset','whole']){
  const result=await page.evaluate(url=>window.rangeProbe(url),storageOrigin+'/object?cap='+token+'&mode='+mode)
  if(mode==='correct'&&(result.size!==fixture.length||JSON.stringify(result.bytes)!==JSON.stringify(Array.from(fixture.subarray(0,32)))))throw new Error('Finite range positive failure')
  if(mode==='hidden'&&result.error!=='CLIP_SOURCE_RANGE_INVALID')throw new Error('Missing exposed Content-Range did not fail')
  if(mode==='wrong-offset'&&result.error!=='CLIP_SOURCE_RANGE_INVALID')throw new Error('Wrong range not rejected')
  if(mode==='whole'&&result.error!=='CLIP_SOURCE_RANGE_UNSUPPORTED')throw new Error('Whole response not rejected')
  report.checks.push({kind:'cross-origin-actual-range-adapter',mode,result});save()
 }
 const policyChecks=[]
 for(const name of Object.keys(policyVariants)){
  const issues=policyIssues(policyVariants[name])
  if(name==='correct'&&issues.length||name!=='correct'&&!issues.length)throw new Error('Policy validation did not distinguish '+name)
  const preflight=await page.request.fetch(storageOrigin+'/object?cap='+token+'&policy='+name,{method:'OPTIONS',headers:{Origin:appOrigin,'Access-Control-Request-Method':'GET','Access-Control-Request-Headers':'range'}})
  const result=await page.evaluate(url=>window.rangeProbe(url),storageOrigin+'/object?cap='+token+'&policy='+name)
  if(name==='correct'&&(preflight.status()!==204||result.error))throw new Error('Configured positive policy failed')
  if(['foreignOnly','missingGet'].includes(name)&&result.error!=='TypeError')throw new Error('Origin/method policy was not enforced '+name)
  if(name==='missingExpose'&&result.error!=='CLIP_SOURCE_RANGE_INVALID')throw new Error('Missing exposed header policy was not enforced')
  if(name==='missingRangeHeader'&&preflight.status()!==403)throw new Error('Missing Range header preflight was not refused')
  policyChecks.push({policy:name,acceptedForRelease:issues.length===0,issues,preflightStatus:preflight.status(),rangeResult:result})
 }
 report.checks.push({kind:'actual-configured-policy-matrix',mapping:{loopbackOrigin:appOrigin,configuredOrigin,foreignOriginsUnmapped:true},policies:policyChecks,scope:'Wildcard may permit a credentialless browser request, but strict private-origin release validation rejects it; single-range GET is CORS-safelisted while explicit Range preflight validates AllowedHeaders.'});save()
 const foreign=await browser.newPage({ignoreHTTPSErrors:true})
 await foreign.goto('https://localhost:'+app.address().port+'/__probe__/');await foreign.waitForFunction(()=>typeof window.rangeProbe==='function')
 const denied=await foreign.evaluate(url=>window.rangeProbe(url),storageOrigin+'/object?cap='+token+'&mode=correct')
 if(denied.error!=='TypeError')throw new Error('Foreign origin unexpectedly accessed private range')
 report.checks.push({kind:'origin-scope-negative',result:denied});await foreign.close()
 const assets=readdirSync(resolve(artifact,'assets'))
 const wasm=assets.find(name=>/^index_bg-.+\.wasm$/.test(name)),worker=assets.find(name=>/^video\.worker-.+\.js$/.test(name)),preview=assets.find(name=>/^preview\.worker-.+\.js$/.test(name))
 if(!wasm||!worker||!preview)throw new Error('Missing current built Worker/WASM')
 const wasmResult=await page.evaluate(async path=>{const r=await fetch(path);const b=await r.arrayBuffer();await WebAssembly.compile(b);return{status:r.status,mime:r.headers.get('content-type'),sha256:Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',b)),x=>x.toString(16).padStart(2,'0')).join('')}},'/assets/'+wasm)
 if(wasmResult.status!==200||wasmResult.mime!=='application/wasm'||wasmResult.sha256!=='22bf6e9f9a100d972da0411a69c5ba504367fc1fa87b3b64e3f35e53926d2d70')throw new Error('Current WASM byte/MIME failure')
 report.checks.push({kind:'actual-built-wasm-compile',asset:wasm,result:wasmResult})
 const workerResult=await page.evaluate(path=>new Promise((resolve,reject)=>{const w=new Worker(path,{type:'module'});const timer=setTimeout(()=>{w.terminate();reject(new Error('Worker timeout'))},10000);w.onmessage=e=>{clearTimeout(timer);w.terminate();resolve(e.data)};w.onerror=e=>{clearTimeout(timer);w.terminate();reject(new Error(e.message))};w.postMessage({type:'start',input:{snapshot:{purpose:'preview'}}})}),'/assets/'+worker)
 if(workerResult.type!=='error'||workerResult.error!=='CLIP_SNAPSHOT_EXPORT_PURPOSE_REQUIRED')throw new Error('Actual built module Worker failed purpose fence')
 report.checks.push({kind:'actual-built-module-worker-execution',asset:worker,result:workerResult,scope:'Known bounded wrong-purpose rejection, not a final-render or performance run.'})
 const previewResult=await page.evaluate(path=>new Promise((resolve,reject)=>{const w=new Worker(path,{type:'module'});const timer=setTimeout(()=>{w.terminate();reject(new Error('Preview Worker timeout'))},10000);w.onmessage=e=>{clearTimeout(timer);w.terminate();resolve(e.data)};w.onerror=e=>{clearTimeout(timer);w.terminate();reject(new Error(e.message))};w.postMessage({type:'initialize',id:1,snapshot:{purpose:'preview'}})}),'/assets/'+preview)
 report.previewProtocolResult=previewResult;save()
 if(previewResult.type!=='failed'||previewResult.error!=='CLIP_SNAPSHOT_INCOMPATIBLE_VERSION:schema')throw new Error('Actual built preview Worker failed malformed-version fence: '+JSON.stringify(previewResult))
 report.checks.push({kind:'actual-built-preview-module-worker-execution',asset:preview,result:previewResult,scope:'Known malformed version rejection, separate from actual mounted-frame/codec evidence.'})
 const inventoryPath=option('--bundle-inventory',null)
 const inventory=inventoryPath?readJSON(resolve(inventoryPath)):null
 const associationFiles=inventory?[...new Set(inventory.mediaChunks), 'assets/'+wasm]:['assets/'+worker,'assets/'+preview,'assets/'+wasm]
 const associations=[]
 for(const file of associationFiles){const r=await page.request.get(appOrigin+'/'+file);const link=r.headers()['link']??'';if(r.status()!==200||!link.includes('/licenses/browser-media/index.html')||!link.includes('/licenses/browser-media/source-access.json'))throw new Error('Missing notice/source association '+file);const bytes=await r.body();if(hash(bytes)!==hash(readFileSync(resolve(artifact,file))))throw new Error('Changed associated media chunk '+file);associations.push({file,sha256:hash(bytes),link,mime:r.headers()['content-type']})}
 report.checks.push({kind:'actual-https-emitted-media-notice-source-association',inventorySourceCommit:inventory?.sourceCommit??null,files:associations})
 const fonts=await page.evaluate(async resources=>{const out=[];for(const f of resources){const r=await fetch('/fonts/clip/'+f.file);const b=await r.arrayBuffer();const h=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',b)),x=>x.toString(16).padStart(2,'0')).join('');const face=await new FontFace(f.family,b,{weight:String(f.weight)}).load();document.fonts.add(face);out.push({file:f.file,bytes:b.byteLength,sha256:h,status:face.status})}return out},fontManifest.resources)
 for(let i=0;i<fonts.length;i++)if(fonts[i].bytes!==fontManifest.resources[i].bytes||fonts[i].sha256!==fontManifest.resources[i].sha256||fonts[i].status!=='loaded')throw new Error('Bundled font byte/load mismatch')
 report.checks.push({kind:'actual-https-font-bytes-and-FontFace-load',fonts,assetVersion:ink.assetVersion})
 const sourceAccess=readJSON(resolve(notices,'source-access.json'))
 const noticeFiles=['index.html','source-access.json','THIRD-PARTY.txt',...sourceAccess.packages.flatMap(p=>p.notices.map(n=>n.file))]
 const fetched=[]
 for(const name of noticeFiles){const path='/licenses/browser-media/'+name;const r=await page.request.get(appOrigin+path);if(r.status()!==200)throw new Error('Unreachable notice '+name);const b=await r.body();if(hash(b)!==hash(readFileSync(resolve(notices,name))))throw new Error('Changed served notice '+name);fetched.push({file:name,bytes:b.length,sha256:hash(b)})}
 report.checks.push({kind:'actual-https-notice-and-source-manifest-access',files:fetched,sourceAccessVerified:sourceAccess.sourceAccessVerified,binaryDependencyGraphVerified:sourceAccess.binaryDependencyGraphVerified})
 const fontNotices=[]
 for(const file of ['LICENSE','LICENSE-Paperlogy','LICENSE-Jua','LICENSE-NanumMyeongjo','INK-NOTICE.txt']){const r=await page.request.get(appOrigin+'/fonts/clip/'+file);if(r.status()!==200)throw new Error('Unreachable font notice '+file);const b=await r.body();if(hash(b)!==hash(readFileSync(resolve(source,'frontend/public/fonts/clip',file))))throw new Error('Changed served font notice '+file);fontNotices.push({file,bytes:b.length,sha256:hash(b)})}
 report.checks.push({kind:'actual-https-original-OFL-and-ink-notice-bodies',files:fontNotices})
 report.requests=requests;report.staticBodies=staticBodies
 report.passed=true;report.limits=['Loopback self-signed HTTPS with browser ignoreHTTPSErrors does not certify a public domain or live Cloudflare/R2 configuration.','The production module Worker proves built URL/execution and wrong-purpose refusal only; previously source-bound media runs remain separate.','Hardware acceleration, semantic preservation, real voice and human output review remain unqualified.','The original npm WASM complete Rust source/license graph remains unresolved.']
 save();console.log(JSON.stringify({passed:true,sourceRevision:revision,browser:report.browser,checks:report.checks.length,report:resolve(output,'report.json'),qualified:false}))
}catch(error){report.error=error.message;save();throw error}
finally{await browser.close();await new Promise(resolve=>app.close(resolve));await new Promise(resolve=>storage.close(resolve))}
