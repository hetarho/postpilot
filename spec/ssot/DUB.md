# DUB spoken voices and narration
> r2 | Create, audition and confirm reusable account-owned spoken voices, then generate narration independently of displayed captions.

## decisions
- DUB-1 [o] a spoken voice is an account-owned sound identity used for generated narration; it is separate from VOICE's writing-style profile and is named 목소리 in the product
- DUB-2 [o] v1 creates Korean-speaking custom voices from a text description; a narrated clip uses one confirmed spoken voice
- DUB-3 [o] every spoken voice, candidate, audition sample and generated speech is private to its authenticated owner; another account cannot read, select, change or reuse it
- DUB-4 [o] 목소리 만들기 accepts a name, desired voice characteristics and an audition text that the owner may edit before generation
  - the starting audition text covers Korean names, numbers, prices, units and mixed-language terms
  - the owner may use their own representative text instead
- DUB-5 [o] candidate generation is an explicit action after an estimate and approval of its bounded AI work; entering or editing fields, opening the page and saving metadata never implicitly generates audio
- DUB-6 [o] generated candidates remain individually playable and selectable; confirming saves the chosen candidate's voice identity, name and audition sample without substituting another voice
- DUB-7 [o] a voice becomes available for clip dubbing only after the owner has auditioned, selected and confirmed it; an unfinished voice is never silently used as a default
- DUB-8 [o] confirmed voices appear in the owner's account-level 목소리 list with their name and playable sample and may be reused across that account's clips
- DUB-9 [o] changing a confirmed voice's sound identity creates a separate candidate set and separately confirmed voice; it never changes the sound identity used by existing speech or clips
- DUB-10 [o] removing a voice from the list prevents its selection for new speech while preserving already generated narration and rendered videos
  - a clip that needs new speech must select a usable confirmed voice
  - removing a voice never regenerates or erases an existing clip's sound or result
- DUB-11 [o] the voice-creation screen shows and selects its eligible generation model; the video editor chooses a confirmed voice with its sample without requiring voice creation or voice-generation model selection again
- DUB-12 [o] each narration generation uses the sound identity the owner confirmed; compatibility refusal is reported before an incompatible provider call, without silently changing the voice, model or supplier
- DUB-13 [o] replaying stored candidates, audition samples or existing speech invokes no synthesis and spends no speech-generation credits
- DUB-14 [o] requesting different candidates or new speech is separately explicit provider work with an estimate; applicable candidate creation, voice confirmation and speech work must fit its approved scope and ceiling
- DUB-15 [o] speech admission accounts for the selected model's documented billing units and enforceable input/output bounds; unknown applicable prices or an unenforceable ceiling refuse the paid work before execution
  - text-token prices are not assumed to price speech
  - compatible reused voices and speech are not synthesized or charged again
- DUB-16 [o] confirmed usage, unused reservation returns, cancellation and failures settle under QUOTA-15, QUOTA-46, QUOTA-49 and QUOTA-52; the final debit never exceeds the approved ceiling
- DUB-17 [o] failed or cancelled candidate/speech generation preserves previously confirmed voices and compatible speech; it exposes the stopped work and an explicit next action without automatic new paid generation
- DUB-18 [o] dubbing is an explicit clip option reachable before first clip generation and in its editor; the narrated workflow selects a confirmed voice, while an ordinary clip requires no spoken voice
- DUB-19 [o] narration reads a dedicated spoken script; displayed captions may contain different wording and do not define the speech input
  - initial captions may be derived as readable segments of the script
  - a display-only text or appearance change leaves the spoken script and its generated speech unchanged
- DUB-20 [o] changing a spoken-script segment marks that segment's speech as requiring regeneration, preserves compatible unchanged speech and presents the changed work for explicit estimated regeneration
  - typing, autosave and preview never initiate that synthesis
  - changing the clip's confirmed voice marks all its narration as requiring regeneration
- DUB-21 [o] updating captions from a changed script is an explicit action that preserves owner-edited displayed text; caption wording updates and timing updates are separate edits
- DUB-22 [o] speech duration and any usable speech timing are measured from the generated audio; initial clip assembly follows that actual timing within CLIP-7's existing output-duration contract
- DUB-23 [o] later speech regeneration preserves owner-edited scenes and caption timing; a duration conflict offers explicit retiming or script revision rather than silently overwriting those edits, cutting speech or changing spoken meaning
- DUB-24 [o] original sound and generated narration have independent owner-controlled volume settings; original-source retention remains governed by CLIP-18
- DUB-25 [o] the draft preview and either export kind use the same confirmed voice, speech and intended timing, including when every original source's audio is disabled
  - an unsupported narration/export path identifies its limitation before work starts
  - the product never represents a file missing requested narration as the narrated result
- DUB-26 [o] a changed script or selected voice cannot be delivered as current while required speech regeneration is unresolved; the earlier result stays identified and playable under the clip result's own revision
- DUB-27 [o] voice creation, audition, confirmation and reuse are delivered and verified before the narrated-video workflow
- DUB-28 [o] v1 includes no translated-source dubbing, recording-based voice cloning, lip synchronization or multiple speakers in one clip

- DUB-29 [o] confirmed sound identity and generated audio are immutable references; renaming changes metadata only, and removing from selection preserves clip-held speech without rebinding the removed voice
  - account deletion removes its private voices, candidates, samples and speech through recoverable cleanup; project deletion removes project-owned speech without deleting reusable account voices
- DUB-30 [o] spoken segments have stable identities and revision-bound speech provenance independent of caption identity; each speech asset identifies the exact script, confirmed voice and admitted profile it represents
  - captions derived from a segment retain that relationship plus independent owner-edit state for displayed text and timing
  - obsolete or late job results cannot replace a newer segment; unchanged compatible audio may be reused without synthesis
- DUB-31 [o] measured audio duration is authoritative even when alignment is absent; incomplete or invalid character timing is not represented as precise word synchronization
  - initial captions use usable timing where present and bounded readable segment timing otherwise; unresolved fit conflicts require DUB-23's explicit choice

## flow
- create voice: 목소리 만들기 → name and voice characteristics → review/edit audition text → estimate and approve candidate work → generate candidates → listen → select → confirm → account 목소리 list
- new candidates: change desired characteristics or audition text → estimate and explicitly request candidates → inspect new candidates → confirm a selected voice; existing confirmed voices remain available
- reuse voice: clip dubbing option → account's confirmed voices and samples → choose → review spoken script → approve remaining speech work → generate speech → inspect with video and independent captions → export
- edit script: change a spoken segment → speech needs regeneration → approve changed work → replace that speech → fit inside retained owner timing, or choose explicit retiming/script revision at a conflict → inspect and export
- edit caption: change displayed wording or appearance → save and preview → existing speech remains reusable
- remove voice: remove from new selection → existing speech and results remain → future speech generation requires a usable confirmed voice

## constraints
- eligibility and entitlement remain server-authoritative under MODEL-16; custom-voice creation and reusable-voice speech need their own qualified capability paths before launch
- provider/model selection and transport, storage representation, segment identity, timing extraction and numeric input limits are engineering decisions; they must preserve the approved voice, bounded costs and independent caption behavior
- qualify the confirmed voice on Korean pronunciation, continuity across different scripts and changed-segment regeneration, usable duration/timing and both clip export kinds before delivery
- clip output duration, original retention and project finalization remain CLIP-owned; confirming a reusable voice does not finalize a video project
- the voice-creation milestone requires the related MODEL and QUOTA policy extensions before its implementation tasks are planned; those extensions do not imply that speech is already supported by the current catalog or text-completion path

## chg
-
