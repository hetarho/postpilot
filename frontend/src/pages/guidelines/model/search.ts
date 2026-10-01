/** `/guidelines?new=1`: open an empty new 지침 at once — where the template request's 지침 만들기
 *  lands (TMPL-61). Anything else is dropped. */
export const guidelinesSearchSchema = (search: Record<string, unknown>): { new?: 1 } => ({
  new: search.new === 1 || search.new === '1' || search.new === true ? 1 : undefined,
})
