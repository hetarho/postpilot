import emptyClips from './empty-clips.png'
import emptyPosts from './empty-posts.png'
import guideline from './kit-guideline.png'
import memory from './kit-memory.png'
import spoken from './kit-spoken.png'
import template from './kit-template.png'
import voice from './kit-voice.png'

/** One drawn set (soft clay, transparent, mid-tone so it sits on both themes). The kit's objects:
 *  a laid-out sheet for templates, a pen for the writing voice, a pinned photo and note for
 *  memories, a ruler and stamp for guidelines, a microphone for the spoken voices. 192 px squares,
 *  shown at 64 px or less. */
export const KIT_ILLUSTRATIONS = { template, voice, memory, guideline, spoken } as const

/** The same set's empty work lists: a notebook for posts, a phone and clapperboard for videos.
 *  256 px squares, shown at 128 px or less. */
export const WORK_ILLUSTRATIONS = { posts: emptyPosts, clips: emptyClips } as const
