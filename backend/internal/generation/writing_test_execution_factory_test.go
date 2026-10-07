package generation

import "testing"

func TestWritingTestExecutionFactoryKeepsOrdinaryModelsAndSharesFrozenConfiguration(t *testing.T) {
	original, replacement := &fakeModels{}, &fakeModels{}
	service := &Service{models: original, batchSize: 4, reasoning: DefaultReasoningPolicy()}
	ports := &plannerPorts{}
	deps := WritingTestFactoryDeps{Sources: ports, Models: ports, Profiles: ports, Templates: ports, Guidelines: ports, Candidates: ports}
	factory := NewWritingTestFactoryWithExecution(service, deps, replacement)
	if factory.service == service || service.models != original || factory.service.models != replacement || factory.service.batchSize != 4 || factory.service.reasoning != service.reasoning {
		t.Fatal("execution adapter changed ordinary service or frozen configuration")
	}
}
