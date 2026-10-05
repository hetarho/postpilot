# Narrated editing and delivery qualification

Date: 2026-10-05. Milestone: T550. Contracts: DUB-25–30, CLIP-200,
MODEL-82 and CDS-101/102.

## Current outcome

Implementation and offline qualification cover the familiar editor, independent
spoken/display text, immutable private speech, draft playback and both MP4
export paths. **Live Korean voice/listening qualification remains unfulfilled.**
No real design, confirmation or synthesis request was made. No production
voice/export readiness was promoted. T539 and T550 retain their live acceptance
gates; passing fixtures cannot replace those gates.

Clip speech admission now requires the profile's separate narrated-export
evidence. Voice-creation evidence alone permits voice creation/reuse only.
Existing stored output access does not resolve a supplier or require a usable
voice handle.

## Automated workflow evidence

| Behavior | Evidence |
| --- | --- |
| Trim, reorder, seek, split and one-step undo | Timeline gesture/split/history tests; actual Chromium responsive checks. |
| Displayed text independent of spoken input | Correction/caption-link tests and `TestClipSpeechLifecycleSourceExpiryRemovedVoiceAndNoSynthesis`; caption edits and replay add no speech request. |
| Only the changed sentence is synthesized | `TestClipSpeechSelectiveReuseAndImmutableTiming`; unchanged immutable assets are retained. |
| Owner caption wording/timing survives | Caption refresh, explicit retiming and narrated-generation tests. |
| Missing alignment | The fixture supplies none; measured audio duration remains authoritative, with no claimed word alignment. |
| Explicit duration conflicts | Readiness/retiming tests retain complete speech and require owner action; rendering rejects unresolved fit. |
| Removed reusable voice | Existing private speech remains playable without rebinding or synthesis; new speech requires a usable qualified voice. |
| Browser output failure | Actual Chromium rejects a changed hash, invalid MP3, unavailable AAC and cancellation, including corrupted speech at gain zero; no RPC/provider call. |
| Billing/count boundaries | Speech idempotency, partial-failure/accounting and browser cancellation/completion tests; browser work does not reserve a successful server-export allowance. |
| Qualification gate | `TestVoiceQualificationAloneCannotEnableNarratedSpeech` and clip admission tests refuse missing export evidence before paid work. |

The complete local CI gates include backend formatting/vet/build/tests,
frontend tests/lint/format/FSD/style/build, deployment recovery tests, generated
proto/SQL reproduction, skill/spec checks and retirement/dev-script checks.
The matching CPU worker image is built and run with local fixture inputs, 1 GiB
memory and two CPUs. Ordinary captions, writing VOICE and ordinary estimates
remain covered by the complete suites.

## Media fixtures

`backend/internal/clip/media/testdata/narration-{660,990}.mp3` are deterministic
two-second stereo marker tones. Their immutable metadata is measured by the
production MP3 inspector. Native gapless decoders preserve every audible sample
and add only bounded trailing codec padding to the canonical measured length.
These files establish timing and audio execution; they establish no Korean
pronunciation, voice identity or continuity.

| Case | Server CPU | Actual Chromium |
| --- | --- | --- |
| 15.8 s vertical, original audio off | Native worker render, decoded markers/order/full content | Web Audio mix, WebCodecs AAC/H.264, stored-file-format MP4 playback/decode |
| 15.8 s square, original audio off | Same fixture contract | Same fixture contract |
| 15.8 s horizontal, source gain 0.25 / speech gain 0.75 | Source-only hook dip, natural speech across a cut boundary | Independent source/speech mix; no dip or rate transform on speech |
| Exact 15 s endpoint | Vertical, last speech ends at the output endpoint | Square, 450 video frames / 720,000 audio frames |
| Exact 60 s endpoint | Horizontal, last speech ends at the output endpoint | Horizontal, 1,800 video frames / 2,880,000 audio frames |

Decoded 15.8-second output comparisons found both 660 Hz and 990 Hz markers at
early and late checkpoints, in the intended order, with no unrequested source
audio. Browser/server tone onset agreed within one 30 fps frame. Independent
gain ratios agreed within 15%; codec/demuxer loudness estimates may differ.
Host FFmpeg measurements were -16.00 LUFS for both source-off paths and
-16.56 / -16.00 LUFS for browser/server mixed output, within the common ±1 LU
contract. True peaks remained below -1.5 dBTP. Browser output decode checks also
cover the late speech in the 15/60-second cases.

Reproduce server fixtures with `pnpm smoke:media-worker`. The actual Chromium
checks used native MP3 decoding, bounded OfflineAudioContext mixing, WebCodecs
AAC/H.264 and MP4 muxing with local synthetic footage. These checks bypassed
remote caption rasters and storage admission; owned storage/provenance and
completion are separately covered by SQLite/integration tests. They do not
claim a real provider-backed end-to-end acceptance run.

## Private asset lifecycle

Private speech has a separate namespace and lifetime from source originals.
The 24-hour source sweep does not remove editable speech. Compatible recordings
remain reusable for the editable project's lifetime, including after a failed
partial generation, cancellation or removal of its reusable voice.

An upload intent is persisted before object I/O. Publication has a bounded
45-second lifetime; cleanup waits at least five minutes. A failed upload,
uncommitted asset row, account/project cascade or restart leaves a durable
retryable intent. No write transaction spans storage I/O. Conditional closure
of a retained intent preserves a concurrent deletion's new intent.

Finalization verifies the exact current speech fingerprint and owned asset
references, keeps the confirmed plan/result's speech, and removes unused speech
only after active job guards have released it. The result remains privately
previewable/downloadable without original access or supplier calls. Confirming
or deleting one clip does not delete an account's reusable voice.

Lifecycle tests cover source expiry, removed voice, project/account deletion,
failed storage cleanup, deletion during a retained read, pre-upload journal
failure, unpublished-upload recovery after restart and a late upload after
terminal cancellation/deletion. Deletion cannot restore playback or publish a
newer segment; cleanup never retries synthesis.

## Responsive and human review

Actual Chromium checks cover 390 px phone and 1440 px desktop layouts in both
day/night themes. The named video/caption/dubbing tracks, trim and order handles,
split action, caption properties, voice/script entry and selective quotation are
reachable. The timeline appears before bulk script-to-caption actions so those
actions cannot push direct editing controls below them. Keyboard edits/undo and
opening controls make no paid call. Earlier
responsive checks also cover 320/360/430/768 px widths and a reduced keyboard
viewport height without horizontal overflow.

These are scripted accessibility/geometry checks, **not** an unprompted human
discoverability study. Physical phones, touch precision, Safari/iOS, Firefox and
other browser/device combinations are unverified. Unsupported decoder/encoder
or memory conditions identify the refused browser path and leave the explicit
server choice available; they never silently omit requested voice.

## Remaining live gates

Use the private voice-creation process in [spoken-voice-v1.md](spoken-voice-v1.md)
first. A subsequent private narrated-clip acceptance must use that exact
confirmed voice for independent spoken/display text, one changed-segment
regeneration, protected owner captions, an explicit duration-conflict decision,
source-off preview and both stored audible exports. Record actual pronunciation
of Korean names, numbers, prices, units, punctuation and mixed-language words,
omissions/repeats, beginnings/endings and continuity against the audition.

Record all ratios and 15/60-second boundaries, source-off and mixed playback,
human discoverability and any unsupported devices. Preserve authorized private
artifacts and the reviewed profile-revision evidence; do not put audio, supplier
handles, account/request identifiers, credentials or supplier prices in this
repository. Enable narrated-export readiness only after both T539 and the real
T550 listening/export criteria pass. Those criteria have not been run here.
