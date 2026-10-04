# IDEATION familiar-video-editing-and-dubbing
> st:converted@261004 | Familiar clip editing with confirmed reusable voices and independently editable narration and captions.

## vision
- problem: The owner cannot identify where or how to edit the generated video; the interface feels unfamiliar compared with other video editors.
- target: Owners creating and refining short videos in Postpilot.
- core value: See what is editable, make predictable changes, and hear the intended narration in the downloaded video.

## explored
- [o] Investigate editing discoverability against familiar video-editor conventions.
- [o] Verify whether a TTS model can generate narration that reaches the final exported video.
- [o] Current editing behavior, verified from the implementation:
  - [ClipTimeline](../../frontend/src/features/correct-clip/ui/ClipTimeline.tsx) renders selectable footage and text bars; it has no bar dragging, edge trimming, timeline-background seeking or draggable playhead.
  - [ClipCorrectionWorkspace](../../frontend/src/features/correct-clip/ui/ClipCorrectionWorkspace.tsx) opens a bottom sheet on selection and suspends the composed preview while it is open. Cut ordering uses up/down buttons; trimming uses a separate source-time range slider and numeric fields in the sheet.
  - Caption positioning can be dragged inside the sheet's [ClipCaptionStage](../../frontend/src/features/correct-clip/ui/ClipCaptionStage.tsx); it is not direct selection and movement in the main preview.
  - Track names are accessibility labels rather than visible track headers. The add-caption button appears only when the plan already has a narration caption.
  - [ClipWorkspace](../../frontend/src/widgets/clip-workspace/ui/ClipWorkspace.tsx) places the storyline space before the preview and timeline when a storyline or region slots exist; the persistent bottom dock carries AI revision requests and render/finalization controls.
- [?] Discoverability hypothesis: The timeline looks like a video-editor surface but behaves as a selector for separate forms; the location of editing tools and the source/output time distinction must be learned before a basic edit.
  - A desktop owner cannot continuously inspect preview, timeline and selected-item controls together in the current sheet-based workflow.
  - Storyline forms and the persistent AI request composer may draw attention away from manual editing; their effect needs confirmation against the owner's actual device and tasks.
- [o] Familiar reference patterns:
  - [Adobe Premiere Properties](https://helpx.adobe.com/premiere/desktop/edit-projects/intro-to-editing/edit-video-using-the-properties-panel.html): Timeline selection exposes contextual clip controls in a properties panel.
  - [CapCut desktop voiceover](https://www.capcut.com/tools/ai-voice-generator-pc): Generate speech from a script, keep it on the editing timeline, and align it with captions and footage before export.
- [o] Current TTS status: Spoken narration generation is absent from the inspected application path.
  - CLIP-134 and CLIP-135 define narration as timed captions; [clip.proto](../../proto/postpilot/v1/clip.proto) has captions and original-source audio settings but no synthesized narration asset or voice setting.
  - [llm.Provider](../../backend/internal/llm/provider.go) returns completion text; [openaicompat.Client](../../backend/internal/llm/openaicompat/client.go) calls chat completions, not speech synthesis.
  - MODEL-13 defines five catalog purposes without speech; MODEL-17 and [catalog fetching](../../backend/internal/modelcatalog/openrouter/client.go) request text/image/video outputs, excluding speech-only models.
  - [Server composition](../../backend/internal/clip/media/composition_render.go) creates audio only from owner-enabled selected originals carrying sound; [browserAudioPlan](../../frontend/src/entities/clip-preview/model/browser-audio-plan.ts) and [browser audio export](../../frontend/src/features/render-clip-browser/api/render-audio.ts) likewise consume original footage audio only.
  - Original sound defaults off under CLIP-18. A captioned video can therefore export with no audio stream.
  - Registering a model alone cannot enable a dubbing workflow.
- [o] TTS feasibility, checked against official documentation on 261004:
  - [OpenRouter TTS](https://openrouter.ai/docs/guides/overview/multimodal/tts) supports speech-output model discovery and a dedicated speech endpoint returning audio; the existing provider can support a future speech workflow, subject to integration and validation.
  - [ElevenLabs models](https://elevenlabs.io/docs/overview/models) document Korean speech support; [speech with timing](https://elevenlabs.io/docs/api-reference/text-to-speech/convert-with-timestamps/) returns audio and character timing, with alignment fields allowed to be null.
  - These establish an available capability, not verified Korean voice quality, installation entitlement or a completed Postpilot export. No paid synthesis call or generated-video playback test was performed.
- [o] Voice creation is the first delivery milestone, completed and confirmed before starting the narrated-video workflow.
  - The owner creates a spoken voice, listens to its sample and confirms it before using it for dubbing.
  - This spoken-voice asset is distinct from VOICE's existing writing-style profile, whose user-facing name is 말투.
  - Proceed with the accepted recommendation of generating a custom voice from a text description, auditioning the candidates and saving the selected voice.
- [o] Description-based custom-voice creation is available as a documented service capability:
  - [ElevenLabs Voice Design](https://elevenlabs.io/docs/eleven-api/guides/how-to/voices/voice-design/) generates audible previews from a description; [create a voice](https://elevenlabs.io/docs/api-reference/text-to-voice/create) saves a selected generated preview for subsequent speech synthesis.
  - This is capability evidence only; no provider or production model is selected by this ideation.
- [o] The speech path preserves the confirmed voice across later scripts and changed-segment regeneration; provider support is a verification requirement, not an assumed existing integration.
- [o] Narration uses a spoken script whose wording may differ from the displayed captions; the owner delegated the practical starting relationship to the recommendation.
  - Initially derive readable caption segments from the script, then allow displayed text to be shortened or rewritten independently.
  - A display-only caption edit leaves the spoken script and existing speech unchanged.
  - Later script edits preserve owner-edited displayed text; an explicit caption refresh updates starting captions without overwriting those edits.
- [o] A changed spoken-script segment shows that its speech needs regeneration; preserve compatible unchanged speech and explicitly generate only the changed work after showing its credit estimate.
  - No typing, autosave or playback automatically initiates paid speech generation.
  - Regenerated speech may have a different duration; preserve manually edited scene/caption timing and offer explicit retiming or script revision when it cannot fit.
- [o] Caption position, color, size and other appearance edits require no new speech generation or speech credits; replaying existing speech likewise requires no new synthesis.
- [o] Original audio and dubbing volume are controlled independently, and the intended dubbing must be the same in the preview and downloaded MP4.
  - Keep original sound off by default under the existing CLIP-18 policy.
  - Verify that both existing export kinds include narration even when original sound is off.
- [o] Adopted editor and voice lifecycle policies:
  - Responsive editor: Support desktop and phone; expose video/caption/dubbing track names, trim/reorder/playhead-split tools, timeline seeking and direct caption selection. Use a persistent contextual panel on desktop and a contextual toolbar with detailed sheets on phone.
  - Empty tracks: Expose first-caption and first-dubbing add actions; a track does not require existing content before it can be edited.
  - AI placement: Generate the first assembled draft and revise an explicit selection; keep storyline/script and AI requests reachable without putting their forms before the existing draft by default.
  - Initial assembly: Use the confirmed voice's actual speech timing for newly generated scenes and captions within the existing 15–60 second contract.
  - Later speech regeneration: Preserve owner-edited scenes and caption timing; if changed speech cannot fit, offer explicit retiming or script revision. Neither owner-edited text nor timing changes without that choice.
  - Duration conflicts: Overlong narration or insufficient usable footage needs an explicit revision choice; do not silently change the chosen duration, footage rate or spoken meaning.
  - Caption wording: Preserve all owner-edited displayed text after script edits; refresh starting caption text through an explicit action that leaves those owner edits in place. Keep this text update separate from timing updates.
  - Voice library: Save confirmed voices at account level for reuse across clips; choosing a different voice in a clip marks all of that clip's speech as requiring regeneration, while existing rendered files remain identified and playable.
  - Voice revision/deletion: Revise a confirmed voice by creating and confirming a separate voice; removing a voice from new selection preserves already generated speech and exported files. A clip that needs new speech must select a usable confirmed voice.
  - Voice/model interface: Voice creation holds the name, desired characteristics, sample text and audible candidates; its selected generation model is visible there. The video editor selects a confirmed voice with its sample rather than making the owner choose a voice-generation model again.
  - Initial scope: Description-generated Korean voices, one confirmed voice per narrated clip, independently editable script/captions, explicit dubbing enablement and selective regeneration.
  - Dubbing activation: Expose the option before first clip generation and inside the editor; starting a narrated workflow selects a confirmed voice, while an ordinary clip can be edited without creating one.
  - Export refusal: Identify unsupported narration/export capability before work starts rather than produce a video missing its narration.
- [o] First-milestone screen: 목소리 만들기 → enter a name and desired voice characteristics → review/edit the audition text → explicitly generate candidates after a credit estimate → listen and select one → confirm and save.
  - The audition text tests Korean names, numbers, prices, units and mixed-language terms; the owner can replace it with their own representative sentence.
  - Replaying generated candidates uses no new speech generation. Requesting different candidates is explicit new provider work with its own estimate.
  - The confirmed voice keeps its name and audition sample so the owner can identify it in later video projects.
- [x] Translation dubbing, recording-based voice cloning, lip synchronization and multiple speakers are deferred from v1 ← first establish confirmed custom voices and a complete single-voice narrated video.
- [o] Engineering verification obligations before delivery:
  - Verify that the voice heard and confirmed during creation is the voice used for each later script and regeneration, and determine which provider paths support that asset.
  - Audition Korean item names, prices, units, punctuation and mixed-language words; define a listenable acceptance sample before choosing a production speech model.
  - Validate actual generated duration and synchronization when timing metadata is absent or incomplete.
  - Verify audible narration in both export kinds and with original audio disabled; document any device limits with an explicit refusal.
  - Confirm speech billing units and enforce an approved ceiling for first generation and changed-paragraph regeneration.

## shape
- flow: Create a spoken voice → audition and confirm it → choose sources and intent → AI assembles a draft with a spoken script and starting captions → review/edit script and captions independently → explicitly generate dubbing with the confirmed voice → inspect video/captions/voice together → export the reviewed video.
- v1: Description-generated Korean voice creation, audition, confirmation and account reuse first; responsive editor, speech-led initial assembly, preserved manual edits, separate script/captions and both narrated exports follow.
- delivery order: Finalize the spoken-voice domain and its acceptance first → implement and verify creation, audition, confirmation and voice reuse → introduce narration and independent captions in the editor → verify narrated preview and both exports.
- validation: The owner can find trim, reorder and caption-edit actions without a walkthrough; speech is audible and synchronized in preview and the downloaded MP4, including a clip with original audio disabled.
- verification boundary: Current-state conclusions come from SSOT/code inspection and official service documentation; no live production UI session, paid TTS trial or new media export has been tested.

## domains
- DUB: Spoken-voice creation, audition, confirmation, account ownership/reuse, speech generation and selected-segment regeneration; separate from VOICE's writing-style learning. →DUB
- CLIP: Editor discoverability, source/script/caption/voice workflow, synchronization, stale speech, and preview/export/finalization behavior. →CLIP
- MODEL: Speech model eligibility, language/voice availability, audition and explicit selection. →MODEL
- QUOTA: Speech generation estimates, reuse, changed-line regeneration and approval/settlement; audio pricing must not be assumed to follow text-token pricing. →QUOTA
- CDS: Voice intelligibility, source-audio balance and identical intended sound across preview and both export kinds. →CDS
- THEME: Desktop and phone editor arrangements, visible track identities and selected-item controls. →THEME

## open
- No unresolved product choices for v1. Provider/model selection, Korean voice quality, custom-voice reuse, billing and both export paths remain engineering verification obligations.
- All domains are converted into their SSOTs; implementation and live qualification remain task work.
