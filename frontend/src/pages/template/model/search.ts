/** `/templates/new?from=<slug>`: the post a new template is asked to follow, attached as the
 *  request's sample (이 글 형식으로 템플릿 만들기, TMPL-64). Anything else is dropped rather than
 *  refused — a hand-edited URL should show the editor, not an error. */
export const newTemplateSearchSchema = (search: Record<string, unknown>): { from?: string } => ({
  from: typeof search.from === 'string' && search.from.trim() !== '' ? search.from : undefined,
})
