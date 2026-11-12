import type { ClipProject, ClipUploadText } from '@/entities/clip-project'

/** The words a clip's checklist reads (CLIP-231): its title, every caption and spoken segment of
 *  the plan, the intro and outro slot text, and the saved 업로드 문구 for its channel. */
export function clipChecklistMaterial(
  project: Pick<ClipProject, 'title' | 'editing' | 'regions'>,
  upload?: Pick<ClipUploadText, 'body' | 'tags'>,
) {
  const plan = project.editing?.plan
  const slots = project.regions
    ? [...project.regions.intro.slots, ...project.regions.outro.slots]
    : []
  return {
    title: project.title,
    captions: (plan?.elements ?? []).flatMap((element) => [
      element.text,
      ...element.rows.map((row) => row.text),
    ]),
    segments: (plan?.narration?.segments ?? []).map((segment) => segment.text),
    regionText: slots.map((slot) => slot.text),
    ...(upload ? { upload: { body: upload.body, tags: upload.tags } } : {}),
  }
}
