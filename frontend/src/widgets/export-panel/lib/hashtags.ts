/** The tags as one ready-to-paste string: `#tag1 #tag2 #tag3`.
 *
 *  A different artifact from the tags the site HTML and the Markdown front matter already embed —
 *  those are a list and a YAML sequence, meant to be read as source. This is what goes into a
 *  platform's own tag box, which is why it is offered on every format tab.
 *
 *  A model asked for tags sometimes writes the `#` itself, so a leading run of them is stripped
 *  and exactly one is put back. An empty result is what the caller keys "render no field" off:
 *  an empty tag list is not an empty control. */
export function toHashtags(tags: readonly string[]): string {
  return tags
    .map((tag) => tag.trim().replace(/^#+/, '').trim())
    .filter((tag) => tag !== '')
    .map((tag) => `#${tag}`)
    .join(' ')
}
