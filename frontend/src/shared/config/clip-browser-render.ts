/** CDS-52 V12. Probe and encode must use the same configurations. */
export const CLIP_BROWSER_RENDER = {
  // Only a reported value below this floor refuses; an unreported value is unknown.
  memoryFloorGB: 2,
  frameRate: 30,
  videoCodec: 'avc1.640028', // AVC High, level 4.0: all three 1080 canvases at 30 fps.
  videoBitrate: 8_000_000,
  audioCodec: 'mp4a.40.2', // AAC-LC.
  audioSampleRate: 48_000,
  audioChannels: 2,
  audioBitrate: 192_000,
} as const
