package main

import (
	"context"
	"database/sql"

	"github.com/postpilot/backend/internal/experiment"
	experimentapp "github.com/postpilot/backend/internal/experiment/app"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// wireWritingTests composes published behavior only. Frozen input, admission,
// execution, target mutation and recovery rules live in their owning contexts.
func wireWritingTests(ctx context.Context, c *contexts) error {
	database, cfg := c.platform.db, c.platform.cfg
	templates := template.NewAuthoring(c.template, templatestore.New(database.Writer, database.Reader))
	guidelines := guideline.NewAuthoring(c.guideline, guidelinestore.New(database.Writer, database.Reader))
	styles := voice.NewAuthoring(c.voice, voicestore.New(database.Writer, database.Reader))
	resolvers := experimentapp.NewWritingTestResolvers(experimentapp.ResolverDependencies{Posts: post.NewWritingTestMaterials(c.post, poststore.New(database.Writer, database.Reader)), Models: c.provider, Voices: c.voice, Templates: templates, Guidelines: c.guideline, GuidelineAuthoring: guidelines, Candidates: c.authoring, Styles: styles})
	deps := generation.WritingTestFactoryDeps{Sources: resolvers, Models: resolvers, Profiles: resolvers, Templates: resolvers, Guidelines: resolvers, Candidates: resolvers}
	execution := experimentapp.NewWritingTestExecutionModels(generationModels{registry: c.metered}, jobstore.New(database.Writer, database.Reader, jobKinds()))
	c.writingTestFactory = generation.NewWritingTestFactoryWithExecution(c.generation, deps, execution)
	preparation := experimentapp.NewWritingTestGeneration(c.writingTestFactory, c.experimentStore)
	runner := experiment.NewWritingTestRunner(c.experimentStore, preparation)
	c.writingTestJobs = experimentapp.NewWritingTestJobs(c.jobs, runner, c.experimentStore, usage.NewSettledCharges(usagestore.New(database.Writer, database.Reader)))
	c.writingTests = experiment.NewWritingTestService(experiment.WritingTestDependencies{Store: c.experimentStore, Variants: resolvers, Preparation: preparation, FailedPreparation: preparation, Pricing: experimentapp.NewWritingTestPricing(c.auth, c.ledger), Queue: c.writingTestJobs}, experiment.WritingTestConfig{Retention: cfg.ExperimentContentRetention})
	guard := experimentapp.NewPostTestJobGuard(func(tx *sql.Tx) experimentapp.OrdinaryWriteJobs { return jobstore.NewTx(tx, jobKinds()) }, ordinaryPostWriteKinds())
	posts := post.NewTestResultService(poststore.NewTestResultStore(database.Writer, database.Reader, guard))
	c.writingTestPublications = experimentapp.NewPublications(experimentapp.PublicationDependencies{Store: c.experimentStore, Snapshots: experimentapp.GenerationPublicationSnapshots{}, Models: provider.NewTestModelAdoptions(c.provider, providerstore.New(database.Writer, database.Reader)), Templates: template.NewTestedSettings(c.template, templatestore.New(database.Writer, database.Reader)), Guidelines: guideline.NewTestedSettings(c.guideline, guidelinestore.New(database.Writer, database.Reader)), Voices: voice.NewTestStylePublisher(voicestore.New(database.Writer, database.Reader)), Posts: posts})
	c.writingTestLifecycle = experimentapp.NewWritingTestLifecycle(c.experimentStore, c.experimentStore, c.writingTestJobs)
	// Deferred jobs must fail before the orphan/terminal hold sweep, including
	// starts whose aggregate bind did not complete before a previous process died.
	if _, err := c.jobs.SweepUnactivated(ctx); err != nil {
		return err
	}
	if _, err := c.jobs.SweepOpenHolds(ctx); err != nil {
		return err
	}
	return c.writingTestLifecycle.Recover(ctx)
}
