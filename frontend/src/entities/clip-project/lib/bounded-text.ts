import { compositionCharacters } from '@/entities/clip-template/@x/clip-project'

/** The longest prefix of a text that fits its maximum, counted CDS-20's way
 *  (CLIP-117). Typing simply stops accepting characters at the bound; a paste
 *  is cut to it, which is the one place characters are dropped — in front of
 *  the counter, rather than behind the owner's back at generation. */
export function boundedText(text: string, max: number) {
  if (compositionCharacters(text) <= max) return text
  const characters = Array.from(text)
  let kept = 0
  for (let i = 0; i < characters.length; i++) {
    if (compositionCharacters(characters[i]) > 0) {
      if (kept === max) return characters.slice(0, i).join('')
      kept += 1
    }
  }
  return text
}
