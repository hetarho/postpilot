package rpc

import (
	"time"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/publishing"
)

func toProtoAgent(agent publishing.Agent) *postpilotv1.PublishingAgent {
	categories := make([]*postpilotv1.PublishingCategory, 0, len(agent.Categories))
	for _, category := range agent.Categories {
		categories = append(categories, &postpilotv1.PublishingCategory{Id: category.ID, Name: category.Name})
	}
	return &postpilotv1.PublishingAgent{Id: agent.ID, Label: agent.Label, Platform: agent.Platform,
		PlatformAccountId: agent.PlatformAccountID, PlatformAccountLabel: agent.PlatformAccountLabel,
		BrowserLabel: agent.BrowserLabel, Categories: categories, DefaultCategoryId: agent.DefaultCategoryID,
		DefaultVisibility: toProtoVisibility(agent.DefaultVisibility), LastSeenAt: formatTime(agent.LastSeenAt),
		RevokedAt: formatTime(agent.RevokedAt), Ready: agent.Ready()}
}

func toProtoJob(job publishing.Job) *postpilotv1.PublishJob {
	return &postpilotv1.PublishJob{Id: job.ID, PostSlug: job.PostSlug, AgentId: job.AgentID, Platform: job.Platform,
		Status: toProtoStatus(job.Status), Stage: toProtoStage(job.Stage), ProgressSequence: job.ProgressSeq,
		ContentRevision: job.ContentRevision, CategoryId: job.CategoryID, Visibility: toProtoVisibility(job.Visibility),
		TargetLanguage: toProtoLanguage(job.TargetLanguage), ContentLanguage: toProtoLanguage(job.ContentLanguage), VoiceSourceLanguage: toProtoLanguage(job.VoiceSourceLanguage),
		Failure: toProtoFailure(job.Failure), PlatformPostUrl: job.PlatformPostURL,
		CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339),
		CommittedAt: formatTime(job.CommittedAt), PublishedAt: formatTime(job.PublishedAt)}
}

func toProtoFailure(failure publishing.Failure) *postpilotv1.Failure {
	if failure.Empty() {
		return nil
	}
	return &postpilotv1.Failure{
		Reason: failure.Reason, Params: failure.Clone().Params, TechnicalDetail: failure.TechnicalDetail,
	}
}

func toProtoManifest(manifest publishing.Manifest, urls []string) *postpilotv1.PublishManifest {
	assets := make([]*postpilotv1.StagedPublishAsset, 0, len(manifest.Assets))
	for i, asset := range manifest.Assets {
		downloadURL := ""
		if i < len(urls) {
			downloadURL = urls[i]
		}
		assets = append(assets, &postpilotv1.StagedPublishAsset{Ordinal: int32(asset.Ordinal), Filename: asset.Filename, SourceFilename: asset.SourceFilename, DownloadUrl: downloadURL, Bytes: asset.Bytes})
	}
	return &postpilotv1.PublishManifest{JobId: manifest.JobID, PostSlug: manifest.PostSlug,
		ContentRevision: manifest.ContentRevision, Content: toProtoContent(manifest.Content), Tags: manifest.Tags,
		CategoryId: manifest.CategoryID, CategoryName: manifest.CategoryName, Visibility: toProtoVisibility(manifest.Visibility),
		TargetLanguage: toProtoLanguage(manifest.TargetLanguage), ContentLanguage: toProtoLanguage(manifest.ContentLanguage), VoiceSourceLanguage: toProtoLanguage(manifest.VoiceSourceLanguage),
		ExpectedPlatformAccountId: manifest.ExpectedPlatformAccountID, Assets: assets}
}

func toProtoLanguage(value publishing.Language) postpilotv1.ContentLanguage {
	switch value {
	case publishing.LanguageKorean:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case publishing.LanguageEnglish:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}

func toProtoContent(content publishing.Content) *postpilotv1.PostContent {
	blocks := make([]*postpilotv1.Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		blocks = append(blocks, &postpilotv1.Block{Type: toProtoBlockType(block.Type), Content: block.Content,
			Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return &postpilotv1.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags, Blocks: blocks}
}

func toProtoBlockType(value publishing.BlockType) postpilotv1.BlockType {
	switch value {
	case publishing.BlockText:
		return postpilotv1.BlockType_TEXT
	case publishing.BlockHeading:
		return postpilotv1.BlockType_HEADING
	case publishing.BlockImage:
		return postpilotv1.BlockType_IMAGE
	case publishing.BlockQuote:
		return postpilotv1.BlockType_QUOTE
	case publishing.BlockList:
		return postpilotv1.BlockType_LIST
	default:
		return postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED
	}
}

func fromProtoVisibility(value postpilotv1.PublishVisibility) publishing.Visibility {
	switch value {
	case postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PUBLIC:
		return publishing.VisibilityPublic
	case postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PRIVATE:
		return publishing.VisibilityPrivate
	default:
		return ""
	}
}

func toProtoVisibility(value publishing.Visibility) postpilotv1.PublishVisibility {
	if value == publishing.VisibilityPrivate {
		return postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PRIVATE
	}
	if value == publishing.VisibilityPublic {
		return postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PUBLIC
	}
	return postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_UNSPECIFIED
}

func fromProtoStage(value postpilotv1.PublishStage) publishing.Stage {
	switch value {
	case postpilotv1.PublishStage_PUBLISH_STAGE_PREPARING:
		return publishing.StagePreparing
	case postpilotv1.PublishStage_PUBLISH_STAGE_OPENING_EDITOR:
		return publishing.StageOpeningEditor
	case postpilotv1.PublishStage_PUBLISH_STAGE_FILLING_CONTENT:
		return publishing.StageFillingContent
	case postpilotv1.PublishStage_PUBLISH_STAGE_UPLOADING_PHOTOS:
		return publishing.StageUploadingPhotos
	case postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING:
		return publishing.StageCommitting
	case postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING:
		return publishing.StageVerifying
	default:
		return ""
	}
}

func toProtoStage(value publishing.Stage) postpilotv1.PublishStage {
	switch value {
	case publishing.StageQueued:
		return postpilotv1.PublishStage_PUBLISH_STAGE_QUEUED
	case publishing.StageClaimed:
		return postpilotv1.PublishStage_PUBLISH_STAGE_CLAIMED
	case publishing.StagePreparing:
		return postpilotv1.PublishStage_PUBLISH_STAGE_PREPARING
	case publishing.StageOpeningEditor:
		return postpilotv1.PublishStage_PUBLISH_STAGE_OPENING_EDITOR
	case publishing.StageFillingContent:
		return postpilotv1.PublishStage_PUBLISH_STAGE_FILLING_CONTENT
	case publishing.StageUploadingPhotos:
		return postpilotv1.PublishStage_PUBLISH_STAGE_UPLOADING_PHOTOS
	case publishing.StageCommitting:
		return postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING
	case publishing.StageVerifying:
		return postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING
	case publishing.StagePublished:
		return postpilotv1.PublishStage_PUBLISH_STAGE_PUBLISHED
	default:
		return postpilotv1.PublishStage_PUBLISH_STAGE_UNSPECIFIED
	}
}

func toProtoStatus(value publishing.Status) postpilotv1.PublishStatus {
	switch value {
	case publishing.StatusQueued:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_QUEUED
	case publishing.StatusRunning:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_RUNNING
	case publishing.StatusPublished:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_PUBLISHED
	case publishing.StatusFailed:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_FAILED
	case publishing.StatusNeedsAttention:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_NEEDS_ATTENTION
	case publishing.StatusOutcomeUnknown:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_OUTCOME_UNKNOWN
	case publishing.StatusCanceled:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_CANCELED
	default:
		return postpilotv1.PublishStatus_PUBLISH_STATUS_UNSPECIFIED
	}
}

func fromProtoFailure(value postpilotv1.PublishFailureKind) publishing.FailureKind {
	switch value {
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_LOGIN_EXPIRED:
		return publishing.FailureLoginExpired
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_CAPTCHA:
		return publishing.FailureCaptcha
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_TWO_FACTOR:
		return publishing.FailureTwoFactor
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_ACCOUNT_MISMATCH:
		return publishing.FailureAccountMismatch
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST:
		return publishing.FailureBrowserLost
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_EDITOR_CHANGED:
		return publishing.FailureEditorChanged
	case postpilotv1.PublishFailureKind_PUBLISH_FAILURE_ASSET_MISSING:
		return publishing.FailureAssetMissing
	default:
		return publishing.FailureSafe
	}
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
