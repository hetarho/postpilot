# Speech audio fixture

`speech-tone.mp3` is a generated 440 Hz sine tone, with no recorded or third-party audio.
Reproduce with:

```sh
ffmpeg -f lavfi -i 'sine=frequency=440:sample_rate=44100:duration=0.25' -ac 2 -c:a libmp3lame -b:a 128k -write_xing 0 -id3v2_version 0 speech-tone.mp3
```

Production validation uses the pure Go decoder. FFmpeg is only a fixture authoring tool.
