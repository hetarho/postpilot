# Synthetic media fixtures

`speech.m4a` is test-only synthesized speech, not a recording of a person or user
media. It was generated locally with macOS `say -v Samantha -r 150`, resampled
to mono 16 kHz, and encoded as AAC at 48 kbit/s with metadata removed.

Text: "Clip preparation keeps every moment. One, two, three. Original audio stays
in time. Four, five, six."

The real-media smoke loops this fixture under deterministic moving/noisy frames,
then compares decoded source/proxy speech at the same timeline offsets. No speech
recognition, network request, external credentials or paid AI is involved.
