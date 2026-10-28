/** `/posts/new?template=<id>` and `/posts/$slug?template=<id>`: a template the wizard — opened
 *  from ①'s 템플릿 picker — just saved, to assign once through the ordinary assignment (TMPL-32).
 *  Anything else is dropped rather than refused, like every page's search. */
export const editorSearchSchema = (search: Record<string, unknown>): { template?: string } => ({
  template:
    typeof search.template === 'string' && search.template.trim() !== ''
      ? search.template
      : undefined,
})
