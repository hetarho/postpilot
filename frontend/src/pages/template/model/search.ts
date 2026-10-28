/** `/templates/new?from=<slug>`: the post a new template is asked to follow, attached as the
 *  request's sample (이 글 형식으로 템플릿 만들기, TMPL-64). `post=<slug|new>`: the post whose
 *  템플릿 picker opened the wizard, which 저장 returns to with the template assigned (TMPL-32).
 *  Anything else is dropped rather than refused — a hand-edited URL should show the editor, not
 *  an error. */
export const newTemplateSearchSchema = (
  search: Record<string, unknown>,
): { from?: string; session?: string; post?: string } => ({
  from: typeof search.from === 'string' && search.from.trim() !== '' ? search.from : undefined,
  session:
    typeof search.session === 'string' && search.session.trim() !== '' ? search.session : undefined,
  post: typeof search.post === 'string' && search.post.trim() !== '' ? search.post : undefined,
})
