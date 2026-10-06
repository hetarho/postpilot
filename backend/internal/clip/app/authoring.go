package app

import (
	"context"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"strings"
)

type Authoring struct {
	service *Service
	store   clip.AuthoringStore
}

func NewAuthoring(service *Service, store clip.AuthoringStore) *Authoring {
	if service == nil || store == nil {
		panic("clip: authoring dependencies required")
	}
	return &Authoring{service: service, store: store}
}
func (a *Authoring) Seed(ctx context.Context, user, id string) (clip.Recipe, string, error) {
	current, err := a.service.store.GetTemplate(ctx, user, id)
	if err != nil {
		return clip.Recipe{}, "", err
	}
	return current.Recipe, clip.AuthoringTemplateVersion(current), nil
}
func (a *Authoring) CanStart(ctx context.Context, user, id string) error {
	if id == "" {
		return nil
	}
	_, err := a.service.store.GetTemplate(ctx, user, id)
	return err
}
func (a *Authoring) Validate(recipe clip.Recipe) (clip.Recipe, error) {
	recipe.Name = strings.TrimSpace(recipe.Name)
	if !clip.BoundedText(recipe.Name, 1, a.service.limits.NameChars) || strings.TrimSpace(recipe.CompositionBody) == "" {
		return clip.Recipe{}, clip.ErrInvalid
	}
	if err := a.service.authoredBody(recipe.CompositionBody); err != nil {
		return clip.Recipe{}, err
	}
	return recipe, nil
}
func (a *Authoring) Guide() string {
	return fmt.Sprintf(`영상 템플릿은 영상 구성만 표현합니다. name은1~%d자, body는%d자까지의 지원 XML입니다. <clip version="1">로 감싸고 <field id="place" label="장소" required="true">어디인가요?</field>처럼 필요한 정보를 받을 수 있습니다. <stage name="방문">방문 장면의 흐름</stage>와 <text id="hook" kind="ai" role="hook"><row kind="ai">영상의 시작</row></text>, <text id="ending" kind="ai" role="ending"><row kind="ai">끝맺음</row></text>을 사용하세요. 각 id는 서로 다른 영문 식별자입니다. scene/repeat나장면별컷·렌더실행정보를템플릿에넣지마세요. intro/outro 디자인선택과 caption style, 기존프로젝트및연결관계를바꾸지마세요. 형식만만들고사용자의실제경험이나사실을꾸며내지마세요.`, a.service.limits.NameChars, a.service.limits.Composition.SourceChars)
}
func (a *Authoring) Publish(ctx context.Context, user string, in clip.AuthoringPublication) (clip.VideoTemplate, error) {
	recipe, err := a.Validate(in.Recipe)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	in.Recipe = recipe
	return a.store.PublishAuthoringTemplate(ctx, user, in, newID(), a.service.now())
}
