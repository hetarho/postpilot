import { GuidelineDirectory } from '@/widgets/guideline-directory'

/** /video-guidelines: a clip's 영상 지침, the same directory as /guidelines (GUIDE-44). */
export function VideoGuidelinesPage() {
  return <GuidelineDirectory kind="clip" />
}
