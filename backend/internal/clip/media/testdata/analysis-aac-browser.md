# Bounded AAC raw-packet regression

These JSON fixtures describe an existing operator-owned T601 synthetic browser copy. They contain packet/frame metadata, no original media, image, speech or PCM samples. They establish decoder/validator compatibility only.

- Existing MP4: 8,291 bytes, SHA256 `facb48b73c141f37edf2d3c4d85a23b6630fb81cc77797a82b1395ba98ad4362`.
- Browser capture: Chrome WebCodecs 154.0.8037.98, T601 source `b3ec132c`; 160×90 H.264 at 15 file FPS, mono AAC 48 kHz, one-second presentation at original offset 60,000 ms.
- Packet EOF: actual pinned CPU ffprobe 7.1 output; host ffprobe 9.0.1 produced the same packet JSON. All 15 video and 49 AAC packets are retained.
- Full presentation decode: host ffprobe 9.0.1, 15 video frames and 48,000 audio samples; frame JSON SHA256 `5e77efbae78ba4472a85ecbd9e1cdbd3aad0e152dfbc6907c8a1fe85f6ba0f0b`.

The raw AAC access units represent `49 × 1024 = 50,176` samples. Presented playback retains 48,000 samples; the difference is 2,112 priming samples plus 64 terminal padding samples. The old two-frame raw-span allowance rejects this normal file. [Apple TN2258](https://developer.apple.com/library/archive/technotes/tn2258/_index.html) documents the relevant 2,112-sample delay and a final incomplete 1,024-sample AAC unit; encoder delay varies by implementation.

The analysis verifier allows at most `2112 + 1023` raw codec-padding samples, retaining its existing timing quantization tolerance. This allowance affects raw AAC endpoints/span only. Packet EOF/cadence/count/size/finite checks, 60-second presented bounds, full decoded sample/time checks, mono 48 kHz, video/SAR/geometry and artifact identity remain independent. The 50th AAC packet in the one-second case exceeds this bound and is refused, alongside hidden 65-second tails, gaps, duplicates, nonfinite timestamps, oversized packets and long packet durations. No packet or audio is removed to pass validation.

Browser semantic qualification remains false. These fixtures cannot certify Korean text/speech preservation, actual supplier sampling, human approval or voice readiness.
