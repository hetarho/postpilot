package job

// Subject is the thing a job belongs to. The dimension is the owning context's own word
// for that thing ("post", "voice", …) and the queue only ever matches on the pair: it
// stores no product vocabulary of its own and learns nothing about what a subject is.
type Subject struct {
	Dimension string
	ID        string
}

func (s Subject) valid() bool { return s.Dimension != "" && s.ID != "" }

// Filter narrows a subject lookup. An empty field means "any".
type Filter struct {
	UserID string
	Kind   string
}

// Guard is one "refuse if work is already active" check Enqueue runs before it inserts.
// The adapter that knows the kind states them, because which subject serializes which
// work is that context's rule rather than the queue's. NewJob with no guard falls back to
// one active job per (user, kind) among rows that carry no post or project — which is what
// unattached work has always been guarded by.
type Guard struct {
	Subject Subject
	Filter  Filter
}

// Subject reports the id this job carries in one dimension, or "" when it carries none.
func (j Job) Subject(dimension string) string { return subjectID(j.Subjects, dimension) }

// Subject reports the id this summary carries in one dimension, or "" when it carries none.
func (s JobSummary) Subject(dimension string) string { return subjectID(s.Subjects, dimension) }

// Subject reports the id this input carries in one dimension, or "" when it carries none.
func (n NewJob) Subject(dimension string) string { return subjectID(n.Subjects, dimension) }

func subjectID(subjects []Subject, dimension string) string {
	for _, s := range subjects {
		if s.Dimension == dimension {
			return s.ID
		}
	}
	return ""
}

func cloneSubjects(subjects []Subject) []Subject {
	if len(subjects) == 0 {
		return nil
	}
	return append([]Subject(nil), subjects...)
}
