const lanes = new Map<string, Promise<unknown>>()

/** Runs one write of a clip project after every write already queued for it has landed.
 *
 *  The settings, the region slots and the correction each save on their own schedule, but the
 *  server moves the plan revision and the region revision on all three (CLIP-188): a slot edit
 *  projects into the plan, a correction edit of a region row changes its slot. Two of them in
 *  flight at once would each carry a revision the other had just moved past. In one lane each
 *  write starts from the answer of the one before — read from the cache that answer replaced —
 *  so none of them sends words the other has already superseded (CLIP-39). A failed write does
 *  not stop the lane; its caller hears the failure. */
export function serialClipWrite<T>(projectId: string, write: () => Promise<T>): Promise<T> {
  const run = (lanes.get(projectId) ?? Promise.resolve()).then(write)
  const tail = run.then(
    () => undefined,
    () => undefined,
  )
  lanes.set(projectId, tail)
  void tail.then(() => {
    if (lanes.get(projectId) === tail) lanes.delete(projectId)
  })
  return run
}
