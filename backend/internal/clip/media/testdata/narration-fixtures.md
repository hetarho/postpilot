# Synthetic narration fixtures

`narration-660.mp3` and `narration-990.mp3` contain two seconds of a 660 Hz and a 990 Hz sine respectively. They contain no speech, account data or supplier output. Both are fixed MPEG1 Layer III, stereo, 44.1 kHz, 128 kbps, encoded with FFmpeg/libmp3lame:

```sh
ffmpeg -f lavfi -i sine=frequency=660:sample_rate=44100:duration=2 -ac 2 -c:a libmp3lame -b:a 128k narration-660.mp3
ffmpeg -f lavfi -i sine=frequency=990:sample_rate=44100:duration=2 -ac 2 -c:a libmp3lame -b:a 128k narration-990.mp3
```

`TestNarratedExportSmoke` obtains hashes, sample counts and format from `llm.InspectSpeechAudio`. It positions the markers at 1.000 and 4.800 seconds, including a transformed cut/transition boundary, and checks their early and late content in the decoded AAC MP4. It checks source-off vertical/square output and mixed horizontal output, retained 330 Hz source sound, immutable result provenance, integrated loudness and true peak. `TestWorkerExecutionParity` separately checks ordinary source-only exports.

The fixed frame count includes codec padding. Gapless browser/FFmpeg decoders may remove at most three MPEG frames of that silence. Canonical playback preserves every decoded sample at its natural rate and pads only trailing silence back to the measured duration. Admission checks that the complete canonical audio fits the delivered CFR interval.

Run in the matching CPU worker image, with network access disabled and the documented memory/CPU limits:

```sh
pnpm smoke:media-worker
```

Optional `CLIP_DIAGNOSTIC_FIXTURES` exports synthetic MP4 artifacts for cross-executor checks. These checks qualify implementation behavior only. They do not qualify Korean pronunciation, a real confirmed voice, a physical device or the production narration profile.
