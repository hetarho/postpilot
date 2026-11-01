/** `/posts/new?template=<id>` and `/posts/$slug?template=<id>`: a template the wizard — opened
 *  from ①'s 템플릿 picker — just saved, to assign once through the ordinary assignment (TMPL-32).
 *  `from=setup`: first-use setup's finish opened this creation, so it offers nothing the owner
 *  just answered (POST-121). Anything else is dropped rather than refused, like every page's
 *  search. */
export const editorSearchSchema = (
  search: Record<string, unknown>,
): { template?: string; from?: 'setup' } => ({
  template:
    typeof search.template === 'string' && search.template.trim() !== ''
      ? search.template
      : undefined,
  from: search.from === 'setup' ? 'setup' : undefined,
})
