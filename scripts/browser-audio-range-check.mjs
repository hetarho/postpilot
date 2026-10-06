import { createReadStream, readFileSync, statSync, mkdirSync, writeFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { execFileSync } from 'node:child_process'
import { chromium } from 'playwright'
import { parseByteRange } from './browser-media-report.mjs'
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..'), args=process.argv.slice(2)
const option=(name,fallback)=>args.includes(name)?args[args.indexOf(name)+1]:fallback
const fixtures=resolve(option('--fixtures','/private/tmp/postpilot-browser-media-prep-audio'))
const output=resolve(option('--output','tmp/browser-audio-ranges'))
mkdirSync(output,{recursive:true})
const files=new Map([['long','stereo48-long.m4a'],['mono','mono44.m4a'],['multiple','multiple-audio-default-second.m4a'],['speech','synthetic-speech.wav'],['multichannel','multichannel.m4a'],['block','unqualified-block.m4a'],['delayed','delayed-audio.mp4']].map(([id,name])=>[id,resolve(fixtures,name)]))
const requests=[], require=createRequire(resolve(root,'frontend/package.json'))
const {createServer}=await import(pathToFileURL(require.resolve('vite')).href)
const server=await createServer({configFile:false,root:resolve(root,'frontend'),cacheDir:resolve(root,'node_modules/.cache/audio-range-check'),resolve:{alias:{'@':resolve(root,'frontend/src')}},server:{host:'127.0.0.1',port:0,hmr:false,watch:null},plugins:[{name:'bounded-audio-fixtures',configureServer(vite){vite.middlewares.use(async(request,response,next)=>{
 response.setHeader('Cross-Origin-Opener-Policy','same-origin');response.setHeader('Cross-Origin-Embedder-Policy','require-corp')
 if(request.url==='/__audio-range__/'){response.setHeader('Content-Type','text/html');response.end('<script type=module src="/src/test/browser-media/audio-range-entry.ts"></script>');return}
 if(request.url?.startsWith('/__audio-range__/output/')&&request.method==='POST'){
  const id=request.url.split('/').at(-1);if(!/^[a-z0-9-]+\.f32$/.test(id)){response.statusCode=400;response.end();return}
  const chunks=[];let size=0;for await(const chunk of request){size+=chunk.length;if(size>16*1024*1024){response.statusCode=413;response.end();return}chunks.push(chunk)}
  writeFileSync(resolve(output,id),Buffer.concat(chunks));response.end('ok');return
 }
 const match=request.url?.match(/^\/__audio-range__\/(file|reference)\/([a-z-]+)$/);if(!match)return next()
 const [,kind,id]=match;if(id==='expired'){response.statusCode=403;response.end('expired');return}
 const path=files.get(id==='whole'?'long':id);if(!path){response.statusCode=404;response.end();return}
 const size=statSync(path).size,range=kind==='file'&&id!=='whole'?parseByteRange(request.headers.range,size):null
 response.writeHead(range?206:200,{'Content-Type':path.endsWith('.wav')?'audio/wav':'audio/mp4','Content-Length':range?range.end-range.start+1:size,'Accept-Ranges':'bytes',...(range?{'Content-Range':`bytes ${range.start}-${range.end}/${size}`}:{})})
 requests.push({kind,id,range,size});const stream=createReadStream(path,range??{});response.on('close',()=>stream.destroy());stream.pipe(response)
})}}]})
await server.listen();const origin=server.resolvedUrls.local[0].replace(/\/$/,'')
const browser=await chromium.launch({executablePath:option('--browser','/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')})
const result=[]
function samples(id){const data=readFileSync(resolve(output,`${id}.f32`));return new Float32Array(data.buffer,data.byteOffset,data.byteLength/4)}
function difference(actual,native){if(actual.length!==native.length)throw Error(`PCM boundaries differ ${actual.length}/${native.length}`);let sum=0,max=0;for(let n=0;n<actual.length;n++){const d=actual[n]-native[n];sum+=d*d;max=Math.max(max,Math.abs(d))}return{frames:actual.length/2,rmse:Math.sqrt(sum/actual.length),maxDifference:max}}
function nativePcm(file,filters,start=0){const data=execFileSync('ffmpeg',['-hide_banner','-loglevel','error',...(start?['-ss',String(start)]:[]),'-i',file,'-map','0:a:0','-af',filters,'-f','f32le','pipe:1'],{maxBuffer:16*1024*1024});return new Float32Array(data.buffer,data.byteOffset,data.byteLength/4)}
function amplitude(data,hz,from=.6,to=.9){let real=0,imaginary=0;const start=Math.round(from*48000),end=Math.min(data.length/2,Math.round(to*48000));for(let n=start;n<end;n++){const phase=2*Math.PI*hz*n/48000;real+=data[2*n]*Math.cos(phase);imaginary+=data[2*n]*Math.sin(phase)}return 2*Math.hypot(real,imaginary)/(end-start)}
try{
 const page=await browser.newPage();await page.goto(`${origin}/__audio-range__/`);await page.waitForFunction(()=>!!window.audioRangeFixture)
 for(const test of [{id:'long-selected',file:'long',startUs:123456000,endUs:125456000},{id:'file-priming',file:'long',startUs:0,endUs:2000000},{id:'mono-clock',file:'mono',startUs:3117000,endUs:5117000},{id:'before-audio-start',file:'delayed',startUs:0,endUs:2000000},{id:'first-audio',file:'multiple',startUs:0,endUs:2000000},{id:'block-refusal',file:'block',startUs:0,endUs:2000000},{id:'channel-refusal',file:'multichannel',startUs:0,endUs:2000000},{id:'memory-refusal',file:'long',startUs:0,endUs:2000000,memoryBytes:128},{id:'range-refusal',file:'whole',startUs:0,endUs:2000000},{id:'expired',file:'expired',startUs:0,endUs:2000000},{id:'decode-cancel',file:'long',startUs:100000000,endUs:120000000,cancel:true}]){
  const request={...test,url:`${origin}/__audio-range__/file/${test.file}`},row=await page.evaluate(request=>window.audioRangeFixture.decode(request),request)
  if(row.resources.liveFrames!==0)throw Error(`AudioData leak: ${JSON.stringify(row)}`)
  if(test.id==='block-refusal'&&row.error!=='CLIP_SOURCE_AUDIO_BLOCK_UNSUPPORTED')throw Error('Missing bounded codec-block refusal')
  if(test.id==='channel-refusal'&&row.error!=='CLIP_SOURCE_AUDIO_CHANNELS_UNSUPPORTED')throw Error('Missing bounded channel-layout refusal')
  if(test.id==='memory-refusal'&&row.error!=='CLIP_SOURCE_AUDIO_MEMORY_LIMIT')throw Error(`Missing memory refusal ${JSON.stringify(row)}`)
  if(test.id==='range-refusal'&&row.error!=='CLIP_SOURCE_RANGE_UNSUPPORTED')throw Error('Missing range refusal')
  if(test.id==='expired'&&row.error!=='CLIP_SOURCE_EXPIRED')throw Error('Missing expiration refusal')
  if(test.cancel){if(!row.error)throw Error('Missing decode cancellation')}
  else if(!test.memoryBytes&&!['whole','expired','multichannel','block'].includes(test.file)){
   if(row.error)throw Error(JSON.stringify(row))
   const native=nativePcm(files.get(test.file),test.file==='delayed'?'aresample=48000:async=1:min_hard_comp=0:first_pts=0,atrim=duration=2,aformat=channel_layouts=stereo,apad,atrim=end_sample=96000':`atrim=start=${test.startUs/1e6}:end=${test.endUs/1e6},asetpts=PTS-STARTPTS,aresample=48000,aformat=channel_layouts=stereo`)
   row.nativeReference=difference(samples(test.id),native)
   if(test.file==='delayed'&&(row.nativeReference.rmse!==0||row.measurements.decodedSamples!==0))throw Error(`Leading source-time silence shifted/decoded: ${JSON.stringify(row)}`)
   if(test.file==='long'&&row.nativeReference.rmse>1e-6)throw Error(`AAC priming/selected drift ${JSON.stringify(row)}`)
   if(test.file==='mono'&&(row.currentReference.rmse>.0005||row.nativeReference.rmse>.005))throw Error(`Resampler clock drift ${JSON.stringify(row)}`)
   if(test.file==='multiple'&&(row.metadata.streamNumber!==1||amplitude(samples(test.id),440)<5*amplitude(samples(test.id),880)))throw Error('First audio stream drift')
  }
  result.push({id:test.id,...row})
 }
 for(const rate of [500,750,1000,1250,1500,2000]){
  const id=`rate-${rate}`,row=await page.evaluate(request=>window.audioRangeFixture.render(request),{id,url:`${origin}/__audio-range__/file/multiple`,rate})
  if(row.error||row.sampleFrames!==row.scheduleFrames||!Number.isFinite(row.loudnessLUFS)||Math.abs(row.loudnessLUFS+16)>1||row.truePeakDBTP>-1.5)throw Error(`Production source render failed ${JSON.stringify(row)}`)
  const expectedFrames=Math.round(Math.round(2000*1000/rate)*30/1000)*1600
  const native=nativePcm(files.get('multiple'),`aresample=48000:async=1:min_hard_comp=0:first_pts=0,atrim=duration=2${rate===1000?'':`,atempo=${rate/1000}`},aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=end_sample=${expectedFrames},asetpts=PTS-STARTPTS`)
  if(row.sampleFrames!==expectedFrames||native.length/2!==expectedFrames)throw Error('Native source sample boundary drift')
  row.pitch={browser440Hz:amplitude(samples(id),440,.2,.8),browser880Hz:amplitude(samples(id),880,.2,.8),native440Hz:amplitude(native,440,.2,.8),native880Hz:amplitude(native,880,.2,.8)}
  if(row.pitch.browser440Hz<10*row.pitch.browser880Hz||row.pitch.native440Hz<10*row.pitch.native880Hz)throw Error(`Pitch changed ${JSON.stringify(row.pitch)}`)
  result.push({id,...row})
 }
 for(const test of [{id:'source-seam',seam:true},{id:'narration-only',narration:true,retain:false},{id:'mixed',narration:true},{id:'mixed-source-low',narration:true,sourceVolume:250},{id:'mixed-speech-low',narration:true,narrationVolume:250},{id:'mixed-hook',narration:true,hook:true},{id:'speech-hash-refusal',narration:true,retain:false,wrongSpeechHash:true},{id:'render-cancel',cancel:true}]){
  const row=await page.evaluate(request=>window.audioRangeFixture.render(request),{...test,url:`${origin}/__audio-range__/file/multiple`})
  if(test.cancel){if(row.name!=='AbortError')throw Error(`Missing production cancellation ${JSON.stringify(row)}`)}
  else if(test.wrongSpeechHash){if(row.error!=='CLIP_BROWSER_AUDIO_AUDIO')throw Error('Speech hash was not refused')}
  else{
   if(row.error||row.sampleFrames!==row.scheduleFrames)throw Error(`Mixed render failure ${JSON.stringify(row)}`)
   if(test.retain===false&&row.sourceCalls!==0)throw Error('Disabled original was opened')
   if(test.narration&&(!row.speechFingerprint||row.speechCalls!==1))throw Error('Immutable speech missing/reloaded')
   if(test.seam&&(row.sampleFrames!==137600||JSON.stringify(row.sourceWindows.map(c=>[c.startSample,c.frames]))!==JSON.stringify([[0,59200],[59200,78400]])))throw Error('Fractional native seam drift')
   row.levels={source:amplitude(samples(test.id),440),speech:amplitude(samples(test.id),770)}
   if(test.seam){
    const graph='[0:a:0]asplit=2[s0][s1];[s0]atrim=start=0:end=1.234,asetpts=PTS-STARTPTS,aresample=48000:async=1:min_hard_comp=0:first_pts=0,aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=end_sample=59200,afade=t=out:st=1.173:d=0.060[a0];[s1]atrim=start=2:end=3.234,asetpts=PTS-STARTPTS,aresample=48000:async=1:min_hard_comp=0:first_pts=0,atempo=0.750000,aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=end_sample=78400,afade=t=in:st=0:d=0.060[a1];[a0][a1]concat=n=2:v=0:a=1,atrim=end_sample=137600,asetpts=PTS-STARTPTS[a]'
    const bytes=execFileSync('ffmpeg',['-hide_banner','-loglevel','error','-i',files.get('multiple'),'-filter_complex',graph,'-map','[a]','-f','f32le','pipe:1'],{maxBuffer:4*1024*1024})
    const native=new Float32Array(bytes.buffer,bytes.byteOffset,bytes.byteLength/4),actual=samples(test.id)
    const rms=(data,from,to)=>{let sum=0;const first=Math.round(from*48000),last=Math.round(to*48000);for(let n=first;n<last;n++)sum+=data[2*n]**2;return Math.sqrt(sum/(last-first))}
    row.nativeSeam={frames:native.length/2,nativeFadeRatio:rms(native,1.193,1.213)/rms(native,.3,.5),browserFadeRatio:rms(actual,1.193,1.213)/rms(actual,.3,.5)}
    if(row.nativeSeam.frames!==137600||Math.abs(row.nativeSeam.nativeFadeRatio-row.nativeSeam.browserFadeRatio)>.05)throw Error('Actual native fractional fade/sample seam drift')
   }
  }
  result.push({id:test.id,...row})
 }
 for(const kind of ['packet','verification']){const row=await page.evaluate(kind=>window.audioRangeFixture.codecRefusal(kind),kind);const expected=kind==='packet'?'AUDIO_PACKET_MEMORY_LIMIT':'AUDIO_VERIFY_MEMORY_LIMIT';if(row.error!==expected)throw Error(`Missing actual codec budget refusal ${JSON.stringify(row)}`);result.push({id:`${kind}-refusal`,...row})}
 const get=id=>result.find(r=>r.id===id),ratio=id=>get(id).levels.speech/get(id).levels.source
 if(Math.abs(ratio('mixed-source-low')/ratio('mixed')-4)>.2||Math.abs(ratio('mixed-speech-low')/ratio('mixed')-250/700)>.03||Math.abs(ratio('mixed-hook')/ratio('mixed')-10**(6/20))>.1)throw Error('Source/speech independent gain or hook dip drift')
 const report={version:1,qualification:false,browser:await browser.version(),node:process.version,nativeFfmpegVersion:execFileSync('ffmpeg',['-version'],{encoding:'utf8'}).split('\n')[0],mediabunny:JSON.parse(readFileSync(resolve(root,'frontend/node_modules/mediabunny/package.json'))).version,lockfileSHA256:createHash('sha256').update(readFileSync(resolve(root,'pnpm-lock.yaml'))).digest('hex'),fixtures:Object.fromEntries([...files].map(([id,path])=>[id,{bytes:statSync(path).size,sha256:createHash('sha256').update(readFileSync(path)).digest('hex')}])),results:result,requests,limits:['Existing local normalizer measures integrated LUFS/true peak; LRA11 and dynamic-gain qualification remain T604.','Synthetic signals do not establish real-voice/listening, identified-device throughput or final T604 release qualification.','Logical PCM/decoder reservations do not measure private codec/GPU/browser heap allocations.','PCM/ALAC/FLAC block layouts and multichannel rematrix remain refused before decoder allocation; actual PCM evidence is AAC mono/stereo.','125ms preroll evidence covers these AAC fixtures; changed-rate resampling is measured separately against current and native references.'],sources:['https://mediabunny.dev/guide/media-sinks','https://mediabunny.dev/guide/reading-media-files']}
 writeFileSync(resolve(output,'report.json'),JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:true,qualification:false,cases:result.length,browser:report.browser,report:resolve(output,'report.json')}))
}finally{await browser.close();await server.close()}
