export interface ClipNotice {
  code: string
  cutId: string
  elementId: string
  action: string
}

// Messages describe delivered content, independent of the failed-attempt vocabulary.
export const clipNoticeKeys = {
  intro_slot_shortened: 'introSlotShortened',
  outro_slot_shortened: 'outroSlotShortened',
  intro_slot_omitted: 'introSlotOmitted',
  outro_slot_omitted: 'outroSlotOmitted',
  composition_contrast: 'contrastReview',
  plan_cut_scene: 'sceneTrimmed',
  plan_cut_rate: 'normalSpeed',
  plan_cut_usability: 'normalSpeed',
  plan_style: 'approvedStyle',
  plan_accent: 'approvedAccent',
  plan_focal: 'cropAdjusted',
  plan_volume: 'volumeAdjusted',
  plan_cut_fade: 'hardCut',
  plan_cut_transition: 'hardCut',
  plan_target_duration: 'durationTrimmed',
  plan_caption_time: 'captionTime',
  composition_cut_evidence: 'evidenceMatched',
  plan_ratio: 'projectRatio',
  composition_section_order: 'sectionOmitted',
  composition_item_order: 'itemOmitted',
  plan_source_overlap: 'overlapOmitted',
  plan_cut_identity: 'duplicateOmitted',
  plan_source: 'sourceOmitted',
  plan_cut_range: 'rangeOmitted',
  plan_source_metadata: 'sourceOmitted',
  composition_cut_identity: 'sourceOmitted',
  composition_observation_gap: 'unobservedOmitted',
  composition_generated_identity: 'textOmitted',
  composition_generated_rows: 'textOmitted',
  composition_generated_bounds: 'longTextOmitted',
  composition_text_shortened: 'textShortened',
  composition_text_omitted: 'longTextOmitted',
  composition_plan_bounds: 'excessCutsOmitted',
  plan_cut_count: 'excessCutsOmitted',
  plan_hook: 'textOmitted',
  plan_chip_count: 'textOmitted',
  plan_chip_label: 'textOmitted',
  plan_copy_chars: 'longTextOmitted',
  plan_copy_classes: 'textOmitted',
  plan_copy_count: 'extraTextOmitted',
  plan_copy_exposure: 'unreadableOmitted',
  plan_copy_format: 'textOmitted',
  plan_copy_keyword: 'textOmitted',
  plan_copy_lines: 'longTextOmitted',
  plan_copy_second_cut: 'extraTextOmitted',
  plan_copy_sequence: 'extraTextOmitted',
  plan_source_audio: 'ownerAudio',
  plan_duration_range: 'durationTrimmed',
  plan_duration_limit: 'durationTrimmed',
  // The narration's own removals: two captions claiming the same moment, an
  // interval the output does not hold, and a window too short to read.
  caption_overlap: 'captionOverlap',
  caption_outside_output: 'captionOutsideOutput',
  caption_floor: 'captionFloor',
  missing_scene_evidence: 'ungroundedText',
  unavailable_scoped_fact: 'ungroundedText',
  unsupported_number_unit: 'ungroundedText',
  unsupported_price_basis: 'ungroundedText',
  unsupported_experience: 'ungroundedText',
  copy_omitted: 'textOmitted',
  repeated_copy: 'repeatedTextOmitted',
  sentence_count: 'extraTextOmitted',
  readability: 'unreadableOmitted',
  rapid_readability: 'unreadableOmitted',
  copy_limit: 'longTextOmitted',
  unsupported_glyph: 'textOmitted',
  automatic_placement: 'textOmitted',
  safe_area: 'textOmitted',
  invalid_interval: 'textOmitted',
  shorter_copy: 'shorterText',
  steady_copy: 'sentencePace',
  short_text: 'shorterText',
  extended_cut: 'extendedCut',
  dropped: 'textOmitted',
  sentence_pace: 'sentencePace',
} as const

export function clipNoticeKey(notice: ClipNotice) {
  if (notice.code === 'plan_target_duration' && notice.action === 'shortfall')
    return 'notices.shorterResult'
  if (notice.code === 'plan_cut_usability' && notice.action === 'removal')
    return 'notices.unusableOmitted'
  if (
    clipNoticeKeys[notice.code as keyof typeof clipNoticeKeys] === 'ungroundedText' &&
    notice.action === 'repair'
  )
    return 'notices.groundedAlternative'
  const key = clipNoticeKeys[notice.code as keyof typeof clipNoticeKeys]
  return key ? (`notices.${key}` as const) : 'inspection.detailUnknown'
}
