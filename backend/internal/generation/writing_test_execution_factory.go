package generation

// NewWritingTestFactoryWithExecution retains the ordinary configured pipeline
// and installs an explicitly required execution port for independently owned
// test jobs. The original service and its ordinary handlers are unchanged.
func NewWritingTestFactoryWithExecution(service *Service, deps WritingTestFactoryDeps, models LLM) *WritingTestFactory {
	if service == nil || models == nil {
		panic("generation: writing test service and execution models are required")
	}
	copy := *service
	copy.models = models
	return NewWritingTestFactory(&copy, deps)
}
