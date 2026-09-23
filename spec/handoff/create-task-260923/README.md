# create-task handoff 260923 (temporary — delete once T322..T351 are written)

create-task QUAL POST GEN GUIDE TMPL was interrupted after the decomposition and before any task file was written.

- `plan.json` — the judged 30-task plan: id (T322..T351, numbered by implementation wave), key (file slug), title, ssot_ids, base_line, dep_ids, goal, scope, out_of_scope. Ids were free at HEAD 28578091; recheck the highest task number before writing.
- `resolutions.md` — binding engineering resolutions R1..R42 that every task must follow (names, enums, RPCs, columns, algorithms, sequencing).

To resume: write each task file from its plan entry (FORMAT task notation, grounded in the real code, answering every question an implementer would ask), add the STATE tasks rows, set QUAL POST GEN GUIDE TMPL `tasked=rev` / pending `-`, next = `implement-task T322`, delete this folder, commit.
