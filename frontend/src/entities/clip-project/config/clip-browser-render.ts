/** CDS-52 V12. Probe and encode must use the same configurations. */
export const CLIP_BROWSER_RENDER = {
  matte: '#000000', // style-escape: encoded video matte and fade-through-black are independent of the UI theme.
  // Only a reported value below this floor refuses; an unreported value is unknown.
  memoryFloorGB: 2,
  frameRate: 30,
  videoCodec: 'avc1.640028', // AVC High, level 4.0: all three 1080 canvases at 30 fps.
  videoBitrate: 8_000_000,
  audioCodec: 'mp4a.40.2', // AAC-LC.
  audioSampleRate: 48_000,
  audioChannels: 2,
  audioBitrate: 192_000,
  audioBatchFrames: 2048,
  encodeQueueFrames: 4,
  keyFrameIntervalFrames: 60,
  sourceTimeoutMs: 30_000,
  // One determinate scale; these are phase weights, never a time estimate.
  encodeProgressPercent: 80,
  storedProgressPercent: 99,
} as const
