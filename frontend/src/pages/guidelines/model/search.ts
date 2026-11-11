/** `/guidelines?new=1`: open an empty new 지침 at once — where the template request's 지침 만들기
 *  lands (TMPL-61). `post=<slug>`: the post whose 확인 opened the page, which its back link returns
 *  to (POST-123 →THEME-58). Anything else is dropped. */
export const guidelinesSearchSchema = (
  search: Record<string, unknown>,
): { new?: 1; post?: string } => ({
  new: search.new === 1 || search.new === '1' || search.new === true ? 1 : undefined,
  post: typeof search.post === 'string' && search.post.trim() !== '' ? search.post : undefined,
})
