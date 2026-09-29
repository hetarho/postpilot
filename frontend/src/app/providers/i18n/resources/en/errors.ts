import type { AppFailureReason } from '@/shared/api'

export const errors = {
  UNKNOWN_FAILURE: 'Could not complete the request. Please try again.',
  AUTH_REQUIRED: 'Log in to continue.',
  INVALID_CREDENTIALS: 'The login ID or password is incorrect.',
  TOO_MANY_ATTEMPTS: 'Too many requests. Try again after {{retry_at, instant}}.',
  GOOGLE_SIGNIN_DISABLED: 'Google sign-in is not available right now.',
  GOOGLE_EMAIL_UNVERIFIED: 'Google did not provide a verified email address.',
  GOOGLE_ACCOUNT_MISMATCH: 'This email is already linked to a different Google account.',
  GOOGLE_SIGNIN_FAILED: 'Could not complete Google sign-in. Please try again.',
  INVALID_EMAIL: 'Check the email address.',
  PASSWORD_TOO_SHORT: 'Enter at least {{min}} characters for the password.',
  PASSWORD_TOO_LONG: 'Keep the password to {{max}} characters or fewer.',
  VERIFICATION_LINK_INVALID: 'The verification link is invalid or has expired.',
  RESET_LINK_INVALID: 'The password reset link is invalid or has expired.',
  EMAIL_ALREADY_VERIFIED: 'This account already has a verified email address.',
  PASSWORD_NOT_SET: 'This account has no password. Use the password reset flow.',
  CURRENT_PASSWORD_WRONG: 'The current password is incorrect.',
  EMAIL_VERIFICATION_REQUIRED: 'Verify your email before registering a payment method.',
  CUSTOMER_KEY_MISMATCH: 'The payment registration does not match the current account.',
  SUBSCRIPTION_NEEDS_METHOD: 'A renewing subscription needs a payment method. Cancel it first.',
  BILLING_UNAVAILABLE: 'Billing is currently unavailable.',
  BILLING_SELECTION_INVALID: 'Check the plan and billing cycle you selected.',
  POST_NOT_FOUND: 'Could not find the post.',
  POST_FORBIDDEN: 'You do not have access to this post.',
  POST_BUSY: 'Another job is already running for this post.',
  POST_CONTENT_STALE: 'This post changed in another screen. Load the latest version and try again.',
  POST_CONTENT_INVALID: 'Check the post content and try again.',
  POST_NOT_FINALIZED: 'Finalize the post first.',
  POST_MACHINE_BASELINE_REQUIRED: 'A saved AI baseline is required.',
  POST_TARGET_LANGUAGE_REQUIRED: 'Select a target language for the post.',
  POST_TARGET_LANGUAGE_UNSUPPORTED: 'That post language is not supported.',
  POST_TEMPLATE_ANSWER_TOO_LONG: 'That is too long. Up to {{max}} characters.',
  POST_TEMPLATE_ANSWER_INVALID: 'Check the fields you filled in.',
  POST_PUBLISHED_LOCKED:
    'A published post cannot be changed. Clear its address on Finish to change it again.',
  POST_PUBLISHED_URL_INVALID:
    'Only a Naver Blog post address can be saved. Paste a post address on blog.naver.com or m.blog.naver.com.',
  POST_FIELD_NOT_FOUND: 'That category is not available. Pick one again.',
  POST_QUALITY_RULE_INVALID: 'The quality rules could not be saved. Check them and try again.',
  POST_LIST_REQUEST_INVALID: 'The post list could not be loaded. Reload the page.',
  VOUCHER_INVALID: 'Check the voucher details and try again.',
  VOUCHER_NOT_FOUND: 'This voucher could not be found. Check the link.',
  VOUCHER_REDEEMED: 'This voucher has already been redeemed.',
  VOUCHER_EXPIRED: 'This voucher link has expired.',
  VOUCHER_REVOKED: 'This voucher has been cancelled.',
  POST_FILENAME_TAKEN: 'A photo with that filename already exists.',
  UPLOAD_INVALID: 'Check the uploaded photo and try again.',
  UPLOAD_NOT_FOUND: 'Could not find the upload.',
  UPLOAD_OBJECT_MISSING: 'Could not find the uploaded photo file.',
  VOICE_REQUIRED: 'Select a voice.',
  VOICE_NOT_FOUND: 'Could not find the voice.',
  VOICE_DELETED: 'This voice has been deleted. Restore it first.',
  VOICE_NOT_MADE: "This voice isn't made yet. Make it or pick another voice.",
  VOICE_NOT_READY: 'Not enough writing yet. Fill what the voice needs to 100%.',
  VOICE_PROMPT_NOT_FOUND: 'That prompt does not exist.',
  VOICE_PROMPT_ANSWERED: 'You already answered this prompt. Delete the answer to write it again.',
  VOICE_ANSWER_REQUIRED: 'Write an answer.',
  VOICE_PHOTO_REQUIRED: 'Choose a photo first.',
  VOICE_NO_PREVIOUS_ANALYSIS: 'There is no previous analysis to return to.',
  VOICE_CHECK_PROMPT_UNANSWERED: 'Answer this prompt first.',
  VOICE_CHECK_PHOTO_UNSUPPORTED:
    "The current writing model can't read photos. Check a photo prompt with a model that reads images.",
  VOICE_CHECK_NOT_FOUND: 'This check could not be found.',
  VOICE_NAME_REQUIRED: 'Enter a voice name.',
  VOICE_NAME_TOO_LONG:
    'The voice name must be no more than {{max}} characters. It is currently {{actual}} characters.',
  VOICE_NAME_TAKEN: 'A voice with that name already exists.',
  VOICE_BUSY: 'A job is already running for this voice.',
  VOICE_SAMPLE_TOO_SHORT:
    'A sample must contain at least {{min}} characters. It currently contains {{actual}}.',
  VOICE_SAMPLE_NOT_FOUND: 'Could not find the voice sample.',
  VOICE_ANALYZE_MODEL_REQUIRED: 'Select a voice-analysis model.',
  VOICE_INVALID_LIFECYCLE: "This action is not available in the voice's current state.",
  CLIP_INVALID_INPUT: 'Check the clip or video template fields and their limits.',
  CLIP_QUOTE_REQUIRED: 'Review and approve the maximum credits before starting generation.',
  CLIP_FINALIZED: 'This clip is finalized. You can download its result.',
  CLIP_FINALIZATION_CONFLICT:
    'The saved draft or result has changed. Review the latest version and render again.',
  CLIP_FINALIZATION_INVALID:
    'No valid result is ready to confirm. Save your draft and render it again.',
  CLIP_CANCELLATION_POLICY_REQUIRED:
    'Refresh to review the cancellation charge before starting a new clip.',
  CLIP_QUOTE_EXPIRED: 'The credit quote expired. Review and approve a new quote.',
  CLIP_QUOTE_CHANGED:
    'The videos, settings or model pricing changed. Review and approve a new quote.',
  CLIP_CREDIT_CEILING_EXCEEDED:
    'Generation did not start: {{required}} credits exceed your approved maximum of {{approved}}. Review a new quote.',
  CLIP_MODEL_PRICING_UNAVAILABLE:
    'Generation did not start because model pricing is unavailable. Choose another model or try again later.',
  CLIP_MODEL_VIDEO_INPUT_ABSENT:
    'The model {{model}} does not take video input, so it cannot analyse clips. Choose a model with video input.',
  CLIP_MODEL_INLINE_ENDPOINT_UNAVAILABLE:
    'No current route for {{model}} accepts the clip video inline. Choose another model or try again later.',
  CLIP_MODEL_REQUIRED_PARAMETERS_UNSUPPORTED:
    'The route for {{model}} does not support the settings the clip analysis request needs. Choose another model.',
  CLIP_MODEL_PRICE_CEILING_UNAVAILABLE:
    'The price ceiling for {{model}} could not be confirmed, so generation did not start. Choose another model or try again later.',
  CLIP_SOURCE_EXPIRED: 'Original retention expired. Reselect matching originals.',
  CLIP_SOURCE_MISSING: 'The original is missing. Reselect the matching file.',
  CLIP_PREVIEW_BUSY: 'Another preview is being prepared. Try again shortly.',
  CLIP_PREVIEW_TOO_LARGE:
    'The preview assets for this interval are too large. Split its text or visibility intervals.',
  CLIP_PREVIEW_UNAVAILABLE: 'Preview preparation is unavailable. Try again shortly.',
  CLIP_PREVIEW_TIMEOUT: 'Preview preparation timed out. Try again shortly.',
  CLIP_SOURCE_UNAVAILABLE:
    'The videos are processing or the upload has expired. Check progress, then select the source videos again.',
  CLIP_NOT_FOUND: 'Could not find the clip or video template.',
  CLIP_COPY_TOO_LONG: 'This caption does not fit in two lines. Shorten it and try again.',
  CLIP_INSUFFICIENT_FOOTAGE:
    'The selected videos are too short to reach the minimum length. Select more footage or lower the target length, then try again.',
  CLIP_LAYOUT_SAFE_AREA:
    'A caption fell outside the screen-safe area, so rendering stopped. Shorten it or move the caption.',
  CLIP_LAYOUT_SIZE:
    'A caption broke the legible minimum size or the line and character limits. Shorten it.',
  CLIP_LAYOUT_OVERLAP:
    'A caption overlapped another element, so rendering stopped. Adjust its position or timing.',
  CLIP_LAYOUT_MOTION: 'A caption used motion that is not allowed, so rendering stopped. Try again.',
  CLIP_LAYOUT_ANCHOR_STEP:
    'Consecutive cuts move their captions too far. Keep the change to one step.',
  CLIP_LAYOUT_FREQUENCY:
    'One caption style was used too often. Bold takes at most two per clip, and one style at most three in a row.',
  CLIP_LAYOUT_DISCLOSURE:
    'The ad disclosure did not cover the whole clip, so rendering stopped. Check the campaign type.',
  CLIP_LAYOUT_KIND: 'The clip contained an element that is not allowed, so rendering stopped.',
  CLIP_LAYOUT_CONTRAST:
    'A caption did not stand out enough from the footage behind it, so rendering stopped. Set that cut to 깔끔하게 or choose a different scene.',
  CLIP_DISCLOSURE_REQUIRED: 'Choose a campaign type first. Every clip carries its ad disclosure.',
  CLIP_TARGET_DURATION_REQUIRED: 'Choose how long the clip should be before generating it.',
  CLIP_BUSY: 'A clip job is still running. Wait for it to finish.',
  CLIP_COMPOSITION_INVALID: 'Check line {{line}} ({{element_id}}) in the video composition.',
  CLIP_COMPOSITION_UNAVAILABLE:
    'Generation for this video composition is being prepared. Your saved composition is preserved.',
  CLIP_PLAN_CONFLICT:
    'The saved edit plan changed. Keep your edits and reload the latest revision before saving again.',
  CLIP_INVALID_MEDIA:
    'The source video could not be verified. Select supported, uncorrupted videos again.',
  CLIP_INPUT_TOO_LARGE:
    'The template, inputs and observations exceed this job’s input allowance. Shorten the template or select fewer sources.',
  CLIP_ANALYSIS_TOO_LARGE:
    'The analysis video could not be prepared within the safe size limit, so AI generation did not start. Check the sources before retrying.',
  CLIP_WORKSPACE_LIMIT:
    'Video processing stopped because temporary storage is insufficient. Your previous result is preserved. Try again later.',
  CLIP_MODEL_INPUT_UNSUPPORTED:
    'The selected model does not support clip video input. Choose a model that supports clip analysis.',
  CLIP_MEDIA_UNAVAILABLE:
    'Video processing could not start. Try again later. Completed observations and your previous result are kept.',
  CLIP_MEDIA_RETRY_EXHAUSTED:
    'Video processing stopped after several attempts. Try again later. Completed observations and your previous result are kept.',
  CLIP_MEDIA_TIMEOUT:
    'Video processing exceeded its time limit. Try again later. Completed observations and your previous result are kept.',
  CLIP_PROCESSING_FAILED:
    'Clip processing failed. Your previous result is preserved. Select the sources again to retry.',
  CLIP_TEMPLATE_NAME_TAKEN: 'A video template with that name already exists.',
  TEMPLATE_NOT_FOUND: 'Could not find the template.',
  PURPOSE_NOT_FOUND: 'Could not find the template you chose. Pick one again.',
  TEMPLATE_NAME_REQUIRED: 'Enter a template name.',
  TEMPLATE_BODY_REQUIRED: 'Enter the template content.',
  TEMPLATE_NAME_TAKEN: 'A template with that name already exists.',
  TEMPLATE_FIELD_TOO_LONG:
    'The value must be no more than {{max}} characters. It is currently {{actual}}.',
  TEMPLATE_NUMBER_OUT_OF_RANGE:
    'The value must be between {{min}} and {{max}}. It is currently {{actual}}.',
  TEMPLATE_PARSE_FAILED: '{{area}} line {{line}}: {{reason}}. Fix it in the source view.',
  TEMPLATE_LIMIT_REACHED:
    'You cannot add another template. Delete one you no longer use and try again.',
  GUIDELINE_NOT_FOUND: 'Could not find the guideline.',
  GUIDELINE_CANDIDATE_NOT_FOUND: 'Could not find the guideline candidate.',
  GUIDELINE_DEFAULT_NOT_FOUND: 'Could not find the default guideline.',
  GUIDELINE_TEXT_REQUIRED: 'Enter the guideline text.',
  GUIDELINE_TEXT_TOO_LONG:
    'A guideline must be no more than {{max}} characters. It is currently {{actual}}.',
  GUIDELINE_TITLE_TOO_LONG:
    'A title can be at most {{max}} characters. It is currently {{actual}}.',
  CLIP_RENDER_NOT_SAMPLED: 'This browser render is still being prepared. Try again shortly.',
  GUIDELINE_TEXT_TAKEN: 'You already have the same guideline.',
  MEMORY_NOT_FOUND: 'Memory not found.',
  MEMORY_TEXT_REQUIRED: 'Enter what to remember.',
  MEMORY_TEXT_TOO_LONG: 'A memory can be at most {{max}} characters. It is {{actual}} now.',
  MEMORY_TEXT_TAKEN: 'You already have that memory.',
  MEMORY_KIND_INVALID: 'Choose a memory kind.',
  MEMORY_TAG_REQUIRED: 'A tag cannot be empty.',
  MEMORY_TAGS_TOO_MANY: 'At most {{max}} tags. There are {{actual}} now.',
  MEMORY_EXTRACTION_NOT_READY: 'No memory candidates yet. Check again in a moment.',
  MEMORY_ANALYZE_MODEL_REQUIRED: 'Register an analyze model before extracting memories.',
  MEMORY_LIMIT_REACHED:
    'You can keep at most {{max}} memories. Delete one you no longer need and try again.',
  GUIDELINE_SCOPE_INVALID:
    'Pick the scope again: leave templates and categories empty for everything, or pick at least one for specific templates or specific categories.',
  GUIDELINE_TEMPLATE_NOT_FOUND:
    'Could not find the template you picked. Refresh the list and try again.',
  GUIDELINE_FIELD_NOT_FOUND: 'That category is not available. Pick one again.',
  GUIDELINE_LIMIT_REACHED:
    'You can save at most {{max}} guidelines. Delete one you no longer use and try again.',
  MODEL_STAGE_REQUIRED: 'Select an AI stage.',
  MODEL_STAGE_INVALID: 'That AI stage is not supported.',
  MODEL_NOT_REGISTERED: 'That model is not registered.',
  MODEL_PURPOSE_INVALID: 'Unknown model purpose. Choose one of the registered purposes.',
  MODEL_PURPOSE_INELIGIBLE: 'That model cannot do this purpose. Choose another model.',
  MODEL_PURPOSE_NOT_REGISTERED:
    'That model is not registered for this purpose. Register it for the purpose first.',
  COMBO_UNKNOWN: 'Unknown estimator combo. Choose one of the four.',
  COMBO_INCOMPLETE: 'Choose the combo and both its analysis and writing models.',
  MODEL_DISABLED: 'That model is disabled.',
  MODEL_UNSUITABLE: 'That model cannot be used for this stage.',
  MODEL_CANDIDATES_DUPLICATE: 'Select two different models.',
  MODEL_RECOMMENDATION_NOT_FOUND: 'Could not find the model recommendation.',
  MODEL_SET_UNAVAILABLE:
    'This recommendation could not be applied: {{models}} cannot be used right now. Choose each stage yourself.',
  MODEL_NOT_FOUND: 'That model could not be found.',
  MODEL_ID_REQUIRED: 'Select a model.',
  MODEL_REASONING_INVALID: 'That reasoning effort is not supported.',
  GENERATION_WRITE_MODEL_REQUIRED: 'Select a writing model.',
  GENERATION_OBSERVE_MODEL_REQUIRED: 'Select a photo-observation model.',
  MODEL_VIDEO_UNSUPPORTED:
    'The selected observation model cannot read this post’s video links. Choose a model that supports signed video URLs.',
  POST_VIDEO_LIMIT: 'This post already holds the maximum number of videos.',
  POST_PHOTO_LIMIT: 'This post already holds the maximum number of photos.',
  UPLOAD_VIDEO_UNSUPPORTED: 'That video format is not supported.',
  UPLOAD_VIDEO_INVALID: 'The video could not be uploaded. Check its length and size.',
  GENERATION_TARGET_LENGTH_INVALID: 'Check the target length.',
  POST_TAG_COUNT_INVALID: 'Check the tag count.',
  POST_TARGET_LENGTH_INVALID: 'Enter a target length between {{min}} and {{max}} characters.',
  POST_PHOTO_MISSING:
    '{{count}} photo places in the text name photos that are no longer attached. Remove those photo blocks in ② or upload the photos again, then finalize.',
  POST_STORYLINE_MISSING: 'This post has no storyline yet. Make one first.',
  POST_STORYLINE_FILE_UNKNOWN: '{{file}} cannot go in this storyline. Make the storyline again.',
  POST_STORYLINE_INVALID: 'Could not save the storyline. Refresh and edit it again.',
  GENERATION_STORYLINE_REOBSERVE:
    'Writing from the storyline takes no photo re-observation choice.',
  CLIP_STORYLINE_MISSING: 'This clip has no storyline yet. Make one first.',
  CLIP_STORYLINE_INVALID:
    'The storyline could not be saved. Keep the same paragraphs, and put each scene in one paragraph only.',
  GENERATION_ALREADY_RUNNING: 'An AI job is already running for this post.',
  GENERATION_VOICE_MISMATCH: 'The selected voice differs from the voice saved on the post.',
  REVISION_INSTRUCTION_REQUIRED: 'Enter a revision request.',
  REVISION_INSTRUCTION_TOO_LONG: 'The revision request must be no more than {{max}} characters.',
  REVISION_CONTENT_REQUIRED: 'Post content is required for a revision.',
  CONTENT_LANGUAGE_REQUIRED: 'The content language is required.',
  EXPERIMENT_NOT_FOUND: 'Could not find the A/B comparison.',
  EXPERIMENT_FORBIDDEN: 'You do not have access to this A/B comparison.',
  EXPERIMENT_STAGE_INVALID: 'An A/B comparison is not available for this stage.',
  EXPERIMENT_MODELS_REQUIRED: 'Select two models to compare.',
  EXPERIMENT_CANDIDATES_DUPLICATE: 'Select two different candidate models.',
  EXPERIMENT_TARGET_LENGTH_INVALID: 'Check the comparison target length.',
  EXPERIMENT_STATE_INVALID: "This action is not available in the comparison's current state.",
  EXPERIMENT_CANDIDATE_NOT_FOUND: 'Could not find the comparison candidate.',
  EXPERIMENT_SNAPSHOT_UNAVAILABLE: 'The saved input required for this comparison is unavailable.',
  EXPERIMENT_RETRY_MODEL_UNAVAILABLE: "The failed candidate's model is no longer available.",
  EXPERIMENT_VOICE_UNAVAILABLE: 'The voice required by this comparison is unavailable.',
  EXPERIMENT_ALREADY_RUNNING: 'Another A/B comparison is already running.',
  EXPERIMENT_POST_FINALIZED: 'A finalized post cannot take a comparison result.',
  EXPERIMENT_BADGES_INVALID: 'The reasons could not be saved. Check them and try again.',
  JOB_NOT_FOUND: 'Could not find the job.',
  JOB_FORBIDDEN: 'You do not have access to this job.',
  JOB_INTERRUPTED: 'The server restarted and interrupted the job.',
  JOB_PANICKED: 'An unexpected problem occurred while running the job.',
  JOB_HANDLER_MISSING: 'The job handler is unavailable.',
  PROVIDER_DISABLED: 'The AI provider is disabled.',
  MODEL_UNAVAILABLE: 'The AI model is temporarily unavailable.',
  MODEL_RATE_LIMITED: 'The AI model is busy. Try again shortly.',
  MODEL_UNSUPPORTED: 'The AI model does not support this operation.',
  MODEL_OUTPUT_INVALID: 'The AI response format could not be read.',
  MODEL_OUTPUT_TRUNCATED: 'The AI response ended before it was complete.',
  TIER_NOT_SUBSCRIBABLE: 'This plan cannot be subscribed to.',
  SUBSCRIPTION_EXISTS: 'There is already an active subscription.',
  SUBSCRIPTION_REQUIRED: 'An active subscription is required first.',
  NO_CHANGE: 'You already use this plan and billing term.',
  NO_SCHEDULED_CHANGE: 'There is no scheduled change to cancel.',
  CHANGE_UNSUPPORTED: 'Change the plan and billing term one at a time.',
  PAYMENT_METHOD_REQUIRED: 'Register a payment method first.',
  CHARGE_FAILED: 'The charge could not be completed. Check your payment method and try again.',
  PURCHASE_TOO_SMALL: 'Credit purchases start at $1.',
  PURCHASE_NOT_FOUND: 'The credit purchase could not be found.',
  REFUND_WINDOW_CLOSED: 'This purchase is more than seven days old and cannot be refunded.',
  PURCHASE_SPENT: 'Some of these purchased credits were used, so the purchase cannot be refunded.',
  REFUND_FAILED: 'The refund could not be completed. Try again shortly.',
  INSUFFICIENT_CREDITS:
    'This needs {{required}} credits and you have {{balance}}. Tops up {{renews_at, instant}}.',
  PLAN_REQUIRED: 'Choose a plan.',
  LAST_MASTER: 'The last operator account cannot be moved to another plan.',
  USER_NOT_FOUND: 'Account not found.',
  USER_ID_REQUIRED: 'Choose an account.',
  MASTER_ONLY: 'This is available to operator accounts only.',
  NETWORK_UNAVAILABLE: 'Could not connect to the network.',
  AI_FX_RATE_UNAVAILABLE:
    'The official exchange rate is temporarily unavailable. Paid AI can resume when a recent rate is confirmed.',
} as const satisfies Record<AppFailureReason, string>
