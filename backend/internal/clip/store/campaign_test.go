package store_test

import (
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/campaign"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"strings"
	"testing"
)

func TestClipCampaignPresenceBoundsAndLegacyDefault(t *testing.T) {
	service, st, database := setup(t)
	_, err := database.Writer.ExecContext(t.Context(), `INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('legacy','alice','legacy','vertical',15000,'2026-10-01T00:00:00Z','2026-10-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := st.GetProject(t.Context(), "alice", "legacy")
	if err != nil || legacy.Campaign || !legacy.AdvertiserGuide.Empty() {
		t.Fatal(legacy, err)
	}
	_, p := create(t, service)
	if !p.Campaign {
		t.Fatal("new project is not campaign content")
	}
	guide := clip.AdvertiserGuide{Text: "  광고주 요청  ", Keywords: []string{"대표 메뉴", "대표메뉴"}, Hashtags: []string{"#제주"}}
	p, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{AdvertiserGuide: &guide})
	if err != nil {
		t.Fatal(err)
	}
	want := clip.AdvertiserGuide{Text: "광고주 요청", Keywords: []string{"대표 메뉴"}, Hashtags: []string{"제주"}}
	if !reflect.DeepEqual(p.AdvertiserGuide, want) {
		t.Fatalf("normalization: %+v", p.AdvertiserGuide)
	}
	off := false
	p, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Campaign: &off})
	if err != nil || p.Campaign || !reflect.DeepEqual(p.AdvertiserGuide, want) {
		t.Fatal(p, err)
	}
	title := "updated"
	p, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Title: &title})
	if err != nil || p.Campaign || !reflect.DeepEqual(p.AdvertiserGuide, want) {
		t.Fatal("omitted fields changed", p, err)
	}
	for _, invalid := range []clip.AdvertiserGuide{{MinChars: new(100)}, {MinPhotos: new(1)}, {Text: strings.Repeat("가", campaign.GuideMaxChars+1)}} {
		_, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{AdvertiserGuide: &invalid, Campaign: new(true), Title: new("must not apply")})
		var tooLong *campaign.TooLongError
		if !errors.Is(err, campaign.ErrItemInvalid) && !errors.As(err, &tooLong) {
			t.Fatal("invalid guide accepted", err)
		}
		got, e := service.GetProject(t.Context(), "alice", p.ID)
		if e != nil || got.Title != title || got.Campaign || !reflect.DeepEqual(got.AdvertiserGuide, want) {
			t.Fatal("refusal was not atomic", got, e)
		}
	}
	if _, err = service.UpdateProject(t.Context(), "bob", p.ID, clip.ProjectPatch{Campaign: new(true)}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign project changed", err)
	}
	empty := clip.AdvertiserGuide{}
	p, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{AdvertiserGuide: &empty})
	if err != nil || !p.AdvertiserGuide.Empty() {
		t.Fatal("clear failed", p, err)
	}
	rows, e := st.ListProjectSummaries(t.Context(), "alice")
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range rows {
		if row.Campaign {
			t.Fatal("summary lost campaign state", row)
		}
	}
	created, e := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "off", Language: "ko", Ratio: "vertical", Campaign: &off, AdvertiserGuide: &want})
	if e != nil || created.Campaign || !reflect.DeepEqual(created.AdvertiserGuide, want) {
		t.Fatal("explicit create off lost", created, e)
	}
}

func TestGenerationFreezesAdvertiserGuide(t *testing.T) {
	for _, on := range []bool{true, false} {
		t.Run(map[bool]string{true: "on", false: "off"}[on], func(t *testing.T) {
			h := generationSetup(t)
			guide := clip.AdvertiserGuide{Text: "広告主の資料", Keywords: []string{"メニュー"}, Hashtags: []string{"体験"}}
			if _, e := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Campaign: &on, AdvertiserGuide: &guide}); e != nil {
				t.Fatal(e)
			}
			id := h.start(t)
			job, e := h.jobs.GetByID(t.Context(), id)
			if e != nil {
				t.Fatal(e)
			}
			var frozen clip.GenerationPayload
			if e = json.Unmarshal(job.Payload, &frozen); e != nil {
				t.Fatal(e)
			}
			if on && (frozen.AdvertiserGuide == nil || !reflect.DeepEqual(*frozen.AdvertiserGuide, guide)) {
				t.Fatal("guide not frozen", string(job.Payload))
			}
			if !on && frozen.AdvertiserGuide != nil {
				t.Fatal("off-state material entered approved job")
			}
		})
	}
}
