import { parseByteRange } from './browser-media-report.mjs'
import { createRequire } from 'node:module'
import { createHash } from 'node:crypto'
import { readFileSync, mkdirSync, writeFileSync, createReadStream, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { chromium } from 'playwright'
const root = resolve(import.meta.dirname, '..'), args = process.argv.slice(2)
const option = (key, fallback) => args.includes(key) ? args[args.indexOf(key) + 1] : fallback
const fixtures = resolve(option('--fixtures', '/private/tmp/postpilot-browser-media-prep-preview/T600-native-layout')), output = resolve(option('--output', '/private/tmp/postpilot-browser-media-prep-preview/T600-browser-editor'))
const manifestPath = resolve(fixtures, 'manifest.json'), manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
const require = createRequire(resolve(root, 'frontend/package.json')), { createServer } = await import(pathToFileURL(require.resolve('vite')).href)
const react = (await import(pathToFileURL(require.resolve('@vitejs/plugin-react')).href)).default, tailwindcss = (await import(pathToFileURL(require.resolve('@tailwindcss/vite')).href)).default
const sourcePath = resolve(option('--source', '/private/tmp/postpilot-browser-media-prep-video/original-cfr60.mp4')), speechPath = resolve(root, 'backend/internal/clip/media/testdata/narration-660.mp3'), requests = []
const server = await createServer({ configFile: false, root: resolve(root, 'frontend'), cacheDir: resolve(root, 'node_modules/.cache/editor-check'), resolve: { alias: { '@': resolve(root, 'frontend/src') } }, server: { host: '127.0.0.1', port: 0, hmr: false, watch: null, headers: { 'Cross-Origin-Opener-Policy': 'same-origin', 'Cross-Origin-Embedder-Policy': 'require-corp' } }, plugins: [react(), tailwindcss(), { name: 'owned-editor-fixture', configureServer(vite) { vite.middlewares.use(async (request, response, next) => {
 if(request.url==='/__editor-fixture__/'){response.setHeader('Content-Type','text/html');response.end(await vite.transformIndexHtml(request.url, '<script type=module src="/src/test/browser-media/editor-entry.tsx"></script>'));return}
 const path=request.url==='/__editor-file__/source'?sourcePath:request.url==='/api/clip/speech/'+ 'a'.repeat(32)?speechPath:undefined;if(!path)return next()
 const size=statSync(path).size,range=parseByteRange(request.headers.range,size);requests.push({speech:path===speechPath,range,size})
 response.writeHead(range?206:200,{'Content-Type':path===speechPath?'audio/mpeg':'video/mp4','Content-Length':range?range.end-range.start+1:size,'Accept-Ranges':'bytes',...(range?{'Content-Range':`bytes ${range.start}-${range.end}/${size}`}:{})});const stream=createReadStream(path,range??{});response.on('close',()=>stream.destroy());stream.pipe(response)
}) } }] })
await server.listen()
const browser = await chromium.launch({ executablePath: option('--browser', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome') })
mkdirSync(output, { recursive: true })
try {
  const page = await browser.newPage(), errors = []
  page.on('pageerror', (error) => { errors.push(error.message); console.error('BROWSER_PAGE_ERROR',error.message) })
  await page.goto(server.resolvedUrls.local[0].replace(/\/$/u, '') + '/__editor-fixture__/')
  await page.waitForFunction(() => !!window.browserEditorFixture)
  const cases = []
  if(!args.includes('--mounted-only')) for (const fixture of manifest.cases) cases.push(await page.evaluate((fixture) => window.browserEditorFixture.layout(fixture), fixture))
  const mounted=[]
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
   const beforeUnmount=await page.evaluate(()=>window.browserEditorFixture.state());await page.evaluate(()=>window.browserEditorFixture.unmount());await page.waitForTimeout(700);const afterUnmount=await page.evaluate(()=>window.browserEditorFixture.state());if(afterUnmount.telemetry.liveNodes!==0)throw Error('Unmount kept source/speech nodes');mounted.push({id:'unmount-release',before:beforeUnmount,after:afterUnmount})

  }
  if (errors.length) throw new Error(JSON.stringify(errors))
  const identity = JSON.parse(readFileSync(resolve(root, 'frontend/src/entities/clip-design/config/clip-ink-identity.json'), 'utf8'))
  const report = { version: 1, sourceFixture:{bytes:statSync(sourcePath).size,sha256:createHash('sha256').update(readFileSync(sourcePath)).digest('hex')}, speechFixture:manifest.speechAssets[0], qualification: false, syntheticOnly: true, competingWork: true, browser: await browser.version(), node: process.version, componentVersion: manifest.componentVersion, assetVersion: identity.assetVersion, nativeManifestSHA256: createHash('sha256').update(readFileSync(manifestPath)).digest('hex'), nativeFixtures: cases.length, cases, mounted, requests, limits: ['Native JSON placements/first-cue boxes are independent expected values; these are actual browser font/WASM/layout checks.', 'Mounted synthetic owned sources exercise actual Worker/codec/font/WASM/control/audio behavior when requested; synthetic tones do not establish real-voice or human/device release qualification.'] }
  writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ passed: true, qualification: false, cases: cases.length, report: resolve(output, 'report.json') }))
} finally { await browser.close(); await server.close() }
