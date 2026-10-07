import { parseByteRange } from './browser-media-report.mjs'
import { createRequire } from 'node:module'
import { createHash } from 'node:crypto'
import { readFileSync, mkdirSync, writeFileSync, createReadStream, statSync, mkdtempSync, rmSync } from 'node:fs'
import { resolve, join } from 'node:path'
import { tmpdir } from 'node:os'
import { spawn, execFileSync } from 'node:child_process'
import { pathToFileURL } from 'node:url'
import { chromium } from 'playwright'
const root = resolve(import.meta.dirname, '..'), args = process.argv.slice(2)
const option = (key, fallback) => args.includes(key) ? args[args.indexOf(key) + 1] : fallback
const fixtures = resolve(option('--fixtures', '/private/tmp/postpilot-browser-media-prep-preview/T600-native-layout')), output = resolve(option('--output', '/private/tmp/postpilot-browser-media-prep-preview/T600-browser-editor'))
const manifestPath = resolve(fixtures, 'manifest.json'), manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
const sourceFence = () => {
 const paths=execFileSync('git',['ls-files','-z','frontend','pnpm-lock.yaml','package.json','scripts/browser-editor-check.mjs','scripts/browser-media-report.mjs','backend/internal/clip/media/browser_layout_fixture_test.go'],{cwd:root,encoding:'utf8'}).split('\0').filter(Boolean).sort()
 return {head:execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8'}).trim(),status:execFileSync('git',['status','--porcelain'],{cwd:root,encoding:'utf8'}).trim(),files:Object.fromEntries(paths.map(path=>[path,createHash('sha256').update(readFileSync(resolve(root,path))).digest('hex')]))}
}
const fenceBefore=sourceFence()
const require = createRequire(resolve(root, 'frontend/package.json')), { createServer } = await import(pathToFileURL(require.resolve('vite')).href)
const react = (await import(pathToFileURL(require.resolve('@vitejs/plugin-react')).href)).default, tailwindcss = (await import(pathToFileURL(require.resolve('@tailwindcss/vite')).href)).default
const sourcePath = resolve(option('--source', '/private/tmp/postpilot-browser-media-prep-video/original-cfr60.mp4')), speechPath = resolve(root, 'backend/internal/clip/media/testdata/narration-660.mp3'), requests = []
const server = await createServer({ configFile: false, root: resolve(root, 'frontend'), cacheDir: resolve(root, 'node_modules/.cache/editor-check'), resolve: { alias: { '@': resolve(root, 'frontend/src') } }, server: { host: '127.0.0.1', port: 0, hmr: false, watch: null, headers: { 'Cross-Origin-Opener-Policy': 'same-origin', 'Cross-Origin-Embedder-Policy': 'require-corp' } }, plugins: [react(), tailwindcss(), { name: 'owned-editor-fixture', configureServer(vite) { vite.middlewares.use(async (request, response, next) => {
 if(request.url==='/__editor-fixture__/'){response.setHeader('Content-Type','text/html');response.end(await vite.transformIndexHtml(request.url, '<script type=module src="/src/test/browser-media/editor-entry.tsx"></script>'));return}
 const path=['/__editor-file__/source','/__editor-file__/saved-result'].includes(request.url)?sourcePath:request.url==='/api/clip/speech/'+ 'a'.repeat(32)?speechPath:undefined;if(!path)return next()
 const size=statSync(path).size,range=parseByteRange(request.headers.range,size);requests.push({url:request.url,speech:path===speechPath,range,size})
 response.writeHead(range?206:200,{'Content-Type':path===speechPath?'audio/mpeg':'video/mp4','Content-Length':range?range.end-range.start+1:size,'Accept-Ranges':'bytes',...(range?{'Content-Range':`bytes ${range.start}-${range.end}/${size}`}:{})});const stream=createReadStream(path,range??{});response.on('close',()=>stream.destroy());stream.pipe(response)
}) } }] })
await server.listen()
let nativeProcess, nativeProfile
let browser
if(args.includes('--native-visibility')) {
 nativeProfile=mkdtempSync(join(tmpdir(),'t600-native-visibility-'))
 nativeProcess=spawn(option('--browser','/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'),['--user-data-dir='+nativeProfile,'--remote-debugging-port=0','--no-first-run','--no-default-browser-check','about:blank'],{stdio:['ignore','ignore','pipe']})
 const endpoint=await new Promise((resolve,reject)=>{let log=''; const timer=setTimeout(()=>reject(Error('Native Chrome launch timeout')),20000);nativeProcess.stderr.on('data',chunk=>{log+=chunk;const match=log.match(/DevTools listening on (ws:\/\/[^\s]+)/);if(match){clearTimeout(timer);resolve(match[1])}});nativeProcess.on('exit',code=>{clearTimeout(timer);reject(Error('Native Chrome exited '+code))})})
 browser=await chromium.connectOverCDP(endpoint,{noDefaults:true})
} else browser=await chromium.launch({ executablePath: option('--browser', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome') })
mkdirSync(output, { recursive: true })
const cases=[],mounted=[],errors=[]
try {
  const page = nativeProcess ? browser.contexts()[0].pages()[0] : await browser.newPage()
  await page.setViewportSize({width:1280,height:900})
  page.on('pageerror', (error) => { errors.push(error.message); console.error('BROWSER_PAGE_ERROR',error.message) })
  await page.goto(server.resolvedUrls.local[0].replace(/\/$/u, '') + '/__editor-fixture__/')
  await page.waitForFunction(() => !!window.browserEditorFixture)
  if(!args.includes('--mounted-only')) for (const fixture of manifest.cases) cases.push(await page.evaluate((fixture) => window.browserEditorFixture.layout(fixture), fixture))
  if(args.includes('--mounted')||args.includes('--mounted-only')){
   const input={url:server.resolvedUrls.local[0].replace(/\/$/u,'')+'/__editor-file__/source',fingerprint:createHash('sha256').update(readFileSync(sourcePath)).digest('hex'),speech:manifest.speechAssets[0]}
   const metadata=await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode:'silent'})
   await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame!==undefined)
   mounted.push({id:'initial-worker',metadata,state:await page.evaluate(()=>window.browserEditorFixture.state())})
   await page.getByRole('button',{name:'Play',exact:true}).click()
   await page.waitForFunction(()=>window.browserEditorFixture.state().timeMs>200)
   await page.getByRole('button',{name:'Pause',exact:true}).click()
   const paused=await page.evaluate(()=>window.browserEditorFixture.state());await page.waitForTimeout(120);const held=await page.evaluate(()=>window.browserEditorFixture.state());if(Math.abs(held.timeMs-paused.timeMs)>1||held.telemetry.liveNodes!==0)throw Error('Pause did not fence the output clock/audio')
   mounted.push({id:'silent-play-pause',paused,held})
   await page.evaluate(()=>window.browserEditorFixture.seek(4500));await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===135)
   await page.getByRole('button',{name:'Flow view',exact:true}).click();await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFlow==='true')
   const flow=await page.evaluate(()=>window.browserEditorFixture.state());await page.waitForTimeout(160);const still=await page.evaluate(()=>window.browserEditorFixture.state());if(flow.timeMs!==still.timeMs||still.telemetry.liveNodes!==0)throw Error('Flow started a clock or audio');mounted.push({id:'still-silent-flow',flow,still})
   await page.getByRole('button',{name:'Video view',exact:true}).click();await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFlow==='false')
   await page.getByRole('button',{name:'현재 장면',exact:true}).click();await page.waitForFunction(()=>!!document.querySelector('button[aria-label="Drag the caption (arrow keys nudge it)"]'))
   mounted.push({id:'selected-controls',state:await page.evaluate(()=>window.browserEditorFixture.state())})
   await page.evaluate(()=>window.browserEditorFixture.seek(1500));await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===45)
   const handle=page.locator('[data-clip-preview-canvas] button[aria-label="Drag the caption (arrow keys nudge it)"]');await handle.focus();await handle.press('ArrowRight');await page.waitForFunction(()=>!!window.browserEditorFixture.state().plan.elements[0].ownerPosition)
   const nudged=await page.evaluate(()=>window.browserEditorFixture.state());await handle.press('Control+z');await page.waitForFunction(()=>!window.browserEditorFixture.state().plan.elements[0].ownerPosition);await handle.press('Control+Shift+z');await page.waitForFunction(()=>!!window.browserEditorFixture.state().plan.elements[0].ownerPosition)
   const box=await handle.boundingBox(),cdp=await page.context().newCDPSession(page),point={x:box.x+box.width/2,y:box.y+box.height/2,id:7,radiusX:8,radiusY:8,force:1}
   await cdp.send('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:1});await page.evaluate(()=>{window.touchEvents=[];for(const type of ['pointerdown','pointermove','pointerup','pointercancel'])document.addEventListener(type,event=>window.touchEvents.push({type,tag:event.target.tagName,label:event.target.getAttribute('aria-label'),x:event.clientX,y:event.clientY,pointerType:event.pointerType}),true)})
   const beforeDrag=await page.evaluate(()=>({state:window.browserEditorFixture.state(),pixels:document.querySelector('[data-clip-local-preview]').toDataURL()}))
   await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[point]});await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{...point,x:point.x+16,y:point.y+8}]});await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipCaptionPosition!=='null')
   const duringDrag=await page.evaluate(()=>({state:window.browserEditorFixture.state(),pixels:document.querySelector('[data-clip-local-preview]').toDataURL()}));if(JSON.stringify(duringDrag.state.plan.elements[0].ownerPosition)!==JSON.stringify(beforeDrag.state.plan.elements[0].ownerPosition)||duringDrag.pixels===beforeDrag.pixels)throw Error('Touch transient did not move the actual frame without committing:'+JSON.stringify({beforeRevision:beforeDrag.state.revision,duringRevision:duringDrag.state.revision,before:beforeDrag.state.plan.elements[0].ownerPosition,during:duringDrag.state.plan.elements[0].ownerPosition,pixelsSame:duringDrag.pixels===beforeDrag.pixels,canvas:duringDrag.state.canvas,touchEvents:await page.evaluate(()=>window.touchEvents),hit:await page.evaluate(p=>({tag:document.elementFromPoint(p.x,p.y)?.tagName,label:document.elementFromPoint(p.x,p.y)?.getAttribute('aria-label')}),point),handles:await handle.evaluate(el=>({touchAction:getComputedStyle(el).touchAction,style:el.getAttribute('style'),rect:el.getBoundingClientRect().toJSON()}))}))
   await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});await page.waitForFunction(before=>JSON.stringify(window.browserEditorFixture.state().plan.elements[0].ownerPosition)!==before,JSON.stringify(beforeDrag.state.plan.elements[0].ownerPosition));await cdp.detach()
   await page.waitForFunction(()=>window.browserEditorFixture.state().rpcCalls.includes('SaveClipEditPlan')&&!window.browserEditorFixture.state().dirty)
   mounted.push({id:'touch-keyboard-undo-autosave',nudged,before:beforeDrag.state,during:duringDrag.state,after:await page.evaluate(()=>window.browserEditorFixture.state())})
   for(const ratio of ['vertical','horizontal','square']) mounted.push({id:'catalog-'+ratio,result:await page.evaluate(ratio=>window.browserEditorFixture.catalog(ratio),ratio)})

   for(const audioMode of ['source','narration','mixed','missing','stale']){
    const metadata=await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode})
    await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame!==undefined)
    const before=await page.evaluate(()=>window.browserEditorFixture.state());await page.getByRole('button',{name:'Play',exact:true}).click();await page.waitForFunction(()=>window.browserEditorFixture.state().timeMs>120)
    await page.getByRole('button',{name:'Pause',exact:true}).click();const after=await page.evaluate(()=>window.browserEditorFixture.state())
    if(after.telemetry.liveNodes!==0)throw Error('Old source/speech remained scheduled after pause')
    if(['narration','mixed'].includes(audioMode)&&after.telemetry.startedNodes<=before.telemetry.startedNodes)throw Error('Ready immutable MP3 was not scheduled')
    if(audioMode==='source'&&metadata.hasAudio&&after.telemetry.startedNodes<=before.telemetry.startedNodes)throw Error('Selected original audio was omitted')
    if(['missing','stale'].includes(audioMode)&&after.telemetry.startedNodes!==before.telemetry.startedNodes)throw Error('Unavailable speech was played')
    mounted.push({id:'clock-'+audioMode,metadata,before,after})
   }

   await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode:'narration'});await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame!==undefined)
   await page.evaluate(()=>window.browserEditorFixture.delays({speech:600}));const beforeLate=await page.evaluate(()=>window.browserEditorFixture.state());await page.getByRole('button',{name:'Play',exact:true}).click();await page.waitForTimeout(80);await page.getByRole('button',{name:'Play',exact:true}).click();await page.waitForTimeout(700);const afterLate=await page.evaluate(()=>window.browserEditorFixture.state());if(afterLate.telemetry.startedNodes!==beforeLate.telemetry.startedNodes||afterLate.timeMs!==beforeLate.timeMs)throw Error('Paused late speech callback started old audio/clock');mounted.push({id:'late-speech-epoch',before:beforeLate,after:afterLate})
   await page.evaluate(()=>window.browserEditorFixture.delays({source:500,speech:0}));await page.getByRole('button',{name:'Refresh preview',exact:true}).click();await page.evaluate(()=>window.browserEditorFixture.seek(2500));await page.evaluate(()=>window.browserEditorFixture.seek(7000));await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===210);await page.waitForTimeout(600);const latest=await page.evaluate(()=>window.browserEditorFixture.state());if(latest.timeMs!==7000||Number(latest.canvas.clipLocalFrame)!==210)throw Error('Old seek/decode replaced current source frame');mounted.push({id:'late-source-epoch',state:latest})
   await page.evaluate(()=>{window.browserEditorFixture.delays({source:0});window.browserEditorFixture.seek(15000)});await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===449);await page.getByRole('button',{name:'Play from the start',exact:true}).click();await page.waitForFunction(()=>window.browserEditorFixture.state().timeMs>80&&window.browserEditorFixture.state().timeMs<1000);await page.getByRole('button',{name:'Pause',exact:true}).click();mounted.push({id:'end-replay',state:await page.evaluate(()=>window.browserEditorFixture.state())})
   await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode:'silent'});await page.evaluate(()=>window.browserEditorFixture.delays({source:900}));await page.waitForFunction(()=>!document.querySelector('button[aria-label="Play"]')?.disabled);await page.waitForTimeout(40);await page.evaluate(()=>window.browserEditorFixture.seek(4000));await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===120);await page.waitForTimeout(950);const measuredAfterSeek=await page.evaluate(()=>window.browserEditorFixture.state());if(Number(measuredAfterSeek.canvas.clipLocalFrame)!==120)throw Error('Aborted initial background measurement poisoned or overwrote same-snapshot seek');mounted.push({id:'background-measurement-cancel-seek',state:measuredAfterSeek})
   await page.evaluate(()=>window.browserEditorFixture.delays({source:0}));for(const frame of [30,45,90,210,449])mounted.push({id:'preview-export-frame-'+frame,result:await page.evaluate(frame=>window.browserEditorFixture.parity(frame),frame)})

   if(nativeProcess) for(const audioMode of ['narration','source']) {
    await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode,sourceDelay:audioMode==='source'?650:0,speechDelay:650})
    await page.waitForFunction(()=>!document.querySelector('button[aria-label="Play"]')?.disabled)
    const before=await page.evaluate(()=>window.browserEditorFixture.state())
    await page.getByRole('button',{name:'Play',exact:true}).click()
    await page.waitForFunction(()=>window.browserEditorFixture.state().telemetry.pendingPlays>0)
    const other=await page.context().newPage();await other.goto('about:blank');await other.bringToFront()
    await page.waitForFunction(()=>document.hidden,{},{polling:50})
    await page.waitForTimeout(850)
    const after=await page.evaluate(()=>({hidden:document.hidden,state:window.browserEditorFixture.state()}))
    if(!after.hidden||after.state.telemetry.liveNodes!==0||after.state.telemetry.startedNodes!==before.telemetry.startedNodes||after.state.telemetry.clockStarts!==before.telemetry.clockStarts||after.state.timeMs!==before.timeMs)throw Error('Hidden deferred '+audioMode+' started old nodes/clock:'+JSON.stringify({before,after}))
    await page.bringToFront();await page.waitForFunction(()=>!document.hidden);await other.close()
    mounted.push({id:'native-hidden-preparing-'+audioMode,before,after})
   }
   await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode:'narration',speechDelay:650});await page.waitForFunction(()=>!document.querySelector('button[aria-label="Play"]')?.disabled)
   const beforeEdit=await page.evaluate(()=>window.browserEditorFixture.state());await page.getByRole('button',{name:'Play',exact:true}).click();await page.waitForFunction(()=>window.browserEditorFixture.state().telemetry.pendingPlays>0)
   await page.evaluate(()=>window.browserEditorFixture.edit({type:'text',id:'caption',patch:{text:'직접 고친 문구'}}));await page.waitForTimeout(800)
   const afterEdit=await page.evaluate(()=>window.browserEditorFixture.state());if(afterEdit.plan.elements[0].text!=='직접 고친 문구'||afterEdit.telemetry.clockStarts!==beforeEdit.telemetry.clockStarts||afterEdit.telemetry.liveNodes!==0)throw Error('Owner edit admitted old pending speech');mounted.push({id:'late-speech-owner-edit',before:beforeEdit,after:afterEdit})
   await page.evaluate(input=>window.browserEditorFixture.mount(input),{...input,audioMode:'silent',rapid:true});await page.waitForFunction(()=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame!==undefined)
   await page.evaluate(()=>window.browserEditorFixture.seek(5000));await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===150)
   const beforeRapid=await page.evaluate(()=>window.browserEditorFixture.state());await page.evaluate(()=>window.browserEditorFixture.edit({type:'text',id:'caption',patch:{phrases:[{text:'주인의 첫 문구',startMs:1000,endMs:3000},{text:'다음 문구',startMs:3000,endMs:8000}]}}));await page.waitForFunction(()=>window.browserEditorFixture.state().plan.elements[0].phrases[0].text==='주인의 첫 문구');await page.waitForFunction(before=>document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFingerprint!==before,beforeRapid.canvas.clipLocalFingerprint)
   mounted.push({id:'rapid-owner-edit',before:beforeRapid,after:await page.evaluate(()=>window.browserEditorFixture.state())})
   mounted.push({id:'blank-preset-slots',result:await page.evaluate(()=>window.browserEditorFixture.blank())})
   await page.evaluate(()=>window.browserEditorFixture.edit({type:'removeText',id:'caption'}));await page.waitForFunction(()=>window.browserEditorFixture.state().plan.elements.length===0);await page.evaluate(()=>window.browserEditorFixture.seek(1500));await page.waitForFunction(()=>Number(document.querySelector('[data-clip-local-preview]')?.dataset.clipLocalFrame)===45)
   mounted.push({id:'caption-delete-current-frame',state:await page.evaluate(()=>window.browserEditorFixture.state())})
   const saved=await page.evaluate(bytes=>window.browserEditorFixture.finalized(bytes),statSync(sourcePath).size);await page.getByRole('tab',{name:'Refine',exact:true}).click();await page.waitForFunction(()=>!!document.querySelector('video'))
   const finalized=await page.evaluate(()=>({state:window.browserEditorFixture.state(),video:document.querySelector('video')?.src,localPreview:!!document.querySelector('[data-clip-local-preview]'),editable:!!document.querySelector('textarea')}))
   if(finalized.video!==saved.resultUrl||finalized.localPreview||finalized.editable||finalized.state.sourceAccessCalls||finalized.state.rpcCalls.some(name=>['PrepareClipPreview','GetClipCaptionPreview','PrepareClipCaptionFrames'].includes(name)))throw Error('Finalized workspace opened draft preview or editable controls:'+JSON.stringify(finalized));mounted.push({id:'actual-finalized-workspace',result:finalized})

   const beforeUnmount=await page.evaluate(()=>window.browserEditorFixture.state());await page.evaluate(()=>window.browserEditorFixture.unmount());await page.waitForTimeout(700);const afterUnmount=await page.evaluate(()=>window.browserEditorFixture.state());if(afterUnmount.telemetry.liveNodes!==0)throw Error('Unmount kept source/speech nodes');mounted.push({id:'unmount-release',before:beforeUnmount,after:afterUnmount})

  }
  if (errors.length) throw new Error(JSON.stringify(errors))
  const identity = JSON.parse(readFileSync(resolve(root, 'frontend/src/entities/clip-design/config/clip-ink-identity.json'), 'utf8'))
  const fenceAfter=sourceFence();if(JSON.stringify(fenceBefore)!==JSON.stringify(fenceAfter))throw Error('Browser check source bytes changed during run')
  const report = { version: 1, sourceFence:{before:fenceBefore,after:fenceAfter,unchanged:true,scope:'All tracked frontend source/public/config plus lock/package, runner dependencies and native fixture exporter; installed dependencies are pinned by the exact lock bytes.'}, nativeVisibility:!!nativeProcess, sourceFixture:{bytes:statSync(sourcePath).size,sha256:createHash('sha256').update(readFileSync(sourcePath)).digest('hex')}, speechFixture:manifest.speechAssets[0], qualification: false, syntheticOnly: true, competingWork: true, browser: await browser.version(), node: process.version, componentVersion: manifest.componentVersion, assetVersion: identity.assetVersion, nativeManifestSHA256: createHash('sha256').update(readFileSync(manifestPath)).digest('hex'), nativeFixtures: cases.length, cases, mounted, requests, limits: ['Native JSON placements/first-cue boxes are independent expected values; these are actual browser font/WASM/layout checks.', 'Mounted synthetic owned sources exercise actual Worker/codec/font/WASM/control/audio behavior when requested; synthetic tones do not establish real-voice or human/device release qualification.', 'Finalized production workspace reads the owned input through a saved-file fixture URL; that UI path does not qualify any finalized export bytes.'] }
  writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ passed: true, qualification: false, cases: cases.length, report: resolve(output, 'report.json') }))
} catch(error) {
 writeFileSync(resolve(output,'failure.json'),JSON.stringify({passed:false,sourceFence:{before:fenceBefore,after:sourceFence()},nativeVisibility:!!nativeProcess,error:String(error),cases,mounted,requests},null,2)+'\n');throw error
} finally {
 if(nativeProcess){await (await browser.newBrowserCDPSession()).send('Browser.close').catch(()=>{});await new Promise(resolve=>{if(nativeProcess.exitCode!==null)return resolve();nativeProcess.once('exit',resolve);setTimeout(()=>{nativeProcess.kill();resolve()},2000)});rmSync(nativeProfile,{recursive:true,force:true})}else await browser.close()
 await server.close()
}
