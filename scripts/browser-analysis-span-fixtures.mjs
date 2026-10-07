import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { createHash } from 'node:crypto'
const root = process.cwd(), outputDir = process.argv[2] ?? '/private/tmp/postpilot-browser-media-prep-t601/source-span'
mkdirSync(outputDir, { recursive: true })
const require = createRequire(resolve(root, 'frontend/package.json'))
const module = await import(pathToFileURL(require.resolve('mediabunny')).href)
const { BlobSource, BufferTarget, EncodedPacket, EncodedPacketSink, EncodedVideoPacketSource, Input, MP4, MkvOutputFormat, Output } = module.default ?? module
const input = new Input({ formats: [MP4], source: new BlobSource(new Blob([readFileSync(resolve(root, 'frontend/src/shared/lib/media/testdata/analysis-silent.mp4'))])) })
try {
  const video = (await input.getVideoTracks())[0], config = await video.getDecoderConfig()
  const first = await new EncodedPacketSink(video).getFirstPacket()
  if (!first || first.type !== 'key') throw new Error('Missing exact synthetic key packet')
  const outputs = {}
  for (const [id, timestamps] of [['sparse-short', [0,62]], ['sparse-missing', [0,62]], ['gapped-vfr', [0,2,5]]]) {
    const target = new BufferTarget(), output = new Output({ format: new MkvOutputFormat(), target })
    const source = new EncodedVideoPacketSource('avc'); output.addVideoTrack(source, { frameRate: 1 })
    await output.start()
    for (const timestamp of timestamps) await source.add(new EncodedPacket(first.data, 'key', timestamp, 1), { decoderConfig: config })
    source.close(); await output.finalize()
    const bytes = new Uint8Array(target.buffer)
    const marker = [0x44,0x89,0x88]
    const duration = bytes.findIndex((_, index) => marker.every((value, n) => bytes[index+n] === value))
    if (duration < 0) throw new Error('Missing standard Segment Duration')
    if (id === 'sparse-short') new DataView(bytes.buffer).setFloat64(duration+3,1000)
    if (id === 'sparse-missing') { bytes[duration]=0xec; bytes[duration+1]=0x89; bytes.fill(0,duration+2,duration+11) }
    const name = `${id}.mkv`; writeFileSync(resolve(outputDir,name),bytes)
    outputs[id] = { bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex'), timestamps, durationHeader: id === 'sparse-short' ? 1 : id === 'sparse-missing' ? null : 6 }
  }
  writeFileSync(resolve(outputDir,'fixtures.json'),JSON.stringify({ version: 1, synthetic: true, mediabunny: '1.58.0', outputs },null,2)+'\n')
  console.log(JSON.stringify({ passed: true, output: outputDir, outputs }))
} finally { input.dispose() }
