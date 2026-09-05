package setup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/postpilot/agent/internal/browser"
	"github.com/postpilot/agent/internal/config"
	"github.com/postpilot/agent/internal/credentials"
	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
	"github.com/postpilot/agent/internal/naver"
	"github.com/postpilot/agent/internal/postpilot"
)

var setupPage = template.Must(template.New("setup-v2").Parse(`<!doctype html>
<html lang="ko"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Postpilot Mac 연결</title>
<style>body{font-family:-apple-system,sans-serif;max-width:640px;margin:auto;padding:32px 16px;background:#17151a;color:#f7f2f8}label{display:block;margin-top:16px}input,select,button{box-sizing:border-box;width:100%;min-height:44px;margin-top:6px;padding:10px 14px;border:0;border-radius:10px;font-size:16px}input,select{background:#29252d;color:#fff}button{background:#8b5cf6;color:#fff;font-weight:600}.secondary{background:#39333e}.message{padding:12px;margin:16px 0;border-radius:10px;background:#193a2b}.error{padding:12px;margin:16px 0;border-radius:10px;background:#48252b}.check{display:flex;gap:10px;align-items:flex-start}.check input{width:44px;margin:8px 0 0}.check span{padding-top:12px}.card{padding:14px;margin:10px 0;border-radius:10px;background:#29252d}.card p{margin:0 0 8px}</style>
<body><h1>Postpilot Mac 연결</h1>
<p>이 서버는 127.0.0.1에만 열려 있습니다. 네이버 비밀번호와 쿠키는 Mac 밖으로 나가지 않습니다.</p>
{{if .Message}}<div class="message">{{.Message}}</div>{{end}}{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Drafts}}<section><h2>미완료 연결</h2><p>연결 코드가 만료되거나 Mac을 재시작해도 이 초안의 전용 브라우저 프로필은 그대로 유지됩니다.</p>
{{range .Drafts}}<form class="card" method="post" action="/setup"><input type="hidden" name="nonce" value="{{$.Nonce}}"><input type="hidden" name="draft_id" value="{{.ID}}"><p><strong>{{.Label}}</strong> · {{.BrowserLabel}}</p><p>{{.APIURL}}</p><button class="secondary" name="action" value="login">같은 전용 네이버 로그인 열기</button><label>현재 연결 코드<input name="device_code" autocomplete="one-time-code"></label><label class="check"><input type="checkbox" name="identity_confirmed" value="yes"><span>이 전용 브라우저에서 네이버 로그인을 마쳤습니다.</span></label><button name="action" value="pair">이 초안으로 연결 완료</button></form>{{end}}</section>{{end}}
{{if .Connections}}<section><h2>기존 연결 로그인 복구</h2><p>로그인 만료, CAPTCHA 또는 2단계 인증이 발생한 연결의 같은 전용 프로필을 다시 엽니다.</p>{{range .Connections}}<form class="card" method="post" action="/setup"><input type="hidden" name="nonce" value="{{$.Nonce}}"><input type="hidden" name="connection_id" value="{{.ID}}"><p><strong>{{.Label}}</strong> · {{.BrowserLabel}}</p>{{if .Status}}<p>{{.Status}}</p>{{end}}<button class="secondary" name="action" value="repair">이 연결의 네이버 로그인 열기</button><button name="action" value="reprobe">로그인 후 호환성 다시 검사</button></form>{{end}}</section>{{end}}
<h2>새 연결</h2><p>새 연결을 누를 때만 별도의 쿠키 저장소가 만들어집니다.</p><form method="post" action="/setup"><input type="hidden" name="nonce" value="{{.Nonce}}"><label>Postpilot API URL<input name="api_url" type="url" required value="{{.Values.APIURL}}"></label><label>연결 이름<input name="label" required value="{{.Values.Label}}"></label><label>전용 브라우저<select name="browser_binary" required>{{range .Browsers}}<option value="{{.Binary}}" {{if eq $.Values.BrowserBinary .Binary}}selected{{end}}>{{.Label}}</option>{{end}}</select></label><button name="action" value="new">새 연결 초안 만들기</button></form></body></html>`))

type Server struct {
	Paths            config.Paths
	Keychain         credentials.Store
	OpenLogin        func(binary, profileDir string) error
	OpenEditor       func(binary, profileDir string) (*browser.Session, error)
	SupportedBrowser func(string) (browser.Installation, bool)
	NewDraftID       func() (string, error)
	Now              func() time.Time
	ProbePublisher   func(context.Context, string) (naver.Result, error)
	Enroll           func(context.Context, string, string, string) (*postpilotv1.EnrollPublishingAgentResponse, error)
	SyncProfile      func(context.Context, string, string, *postpilotv1.SyncAgentProfileRequest) (*postpilotv1.PublishingAgent, error)
	nonce            string
	host             string
	completed        chan struct{}
}

func (s Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	nonce, err := setupNonce()
	if err != nil {
		_ = listener.Close()
		return err
	}
	s.nonce = nonce
	s.host = listener.Addr().String()
	s.completed = make(chan struct{}, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("POST /setup", s.submit)
	server.Handler = mux
	url := "http://" + s.host + "/?nonce=" + url.QueryEscape(s.nonce)
	fmt.Printf("Postpilot agent setup: %s\n", url)
	_ = exec.Command("/usr/bin/open", url).Start()
	go func() {
		select {
		case <-ctx.Done():
		case <-s.completed:
		}
		_ = server.Shutdown(context.Background())
	}()
	err = server.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

type pageData struct {
	Browsers    []browser.Installation
	Connections []repairConnection
	Drafts      []draftView
	Values      formValues
	Message     string
	Error       string
	Nonce       string
}

type repairConnection struct {
	ID, Label, BrowserLabel, Status string
}

type draftView struct {
	ID, Label, BrowserLabel, APIURL string
}

type formValues struct {
	APIURL, DeviceCode, Label, BrowserBinary string
}

func (s Server) index(writer http.ResponseWriter, request *http.Request) {
	if !s.authorize(writer, request, request.URL.Query().Get("nonce")) {
		return
	}
	s.render(writer, pageData{Browsers: browser.Discover(), Values: formValues{APIURL: config.DefaultAPIURL, Label: "내 Mac"}})
}

func (s Server) submit(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
	if request.ParseForm() != nil {
		http.Error(writer, "invalid form", http.StatusBadRequest)
		return
	}
	if !s.authorize(writer, request, request.FormValue("nonce")) {
		return
	}
	action := request.FormValue("action")
	if action != "new" && action != "login" && action != "pair" && action != "repair" && action != "reprobe" {
		http.Error(writer, "invalid action", http.StatusBadRequest)
		return
	}
	if action == "repair" {
		s.repair(writer, request)
		return
	}
	if action == "reprobe" {
		s.reprobe(writer, request)
		return
	}
	values := formValues{APIURL: request.FormValue("api_url"), DeviceCode: request.FormValue("device_code"), Label: request.FormValue("label"), BrowserBinary: request.FormValue("browser_binary")}
	data := pageData{Browsers: browser.Discover(), Values: values}
	if action == "new" {
		s.createDraft(writer, data)
		return
	}
	cfg, err := config.Load(s.Paths)
	if err != nil {
		data.Error = "저장된 연결을 읽지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	draftID := strings.TrimSpace(request.FormValue("draft_id"))
	draft, draftIndex, err := exactDraft(cfg, draftID)
	if err != nil {
		data.Error = err.Error()
		s.render(writer, data)
		return
	}
	values.APIURL, values.Label, values.BrowserBinary = draft.APIURL, draft.Label, draft.BrowserBinary
	data.Values = values
	// Pairing must not create a profile, open or mutate the editor, consume its
	// one-time device code, or advertise ready until the deterministic driver can
	// prove support. Login-only setup remains available for an existing profile.
	if action == "pair" && s.ProbePublisher == nil {
		data.Error = "결정론적 Naver 퍼블리셔가 아직 구현되지 않아 연결을 활성화할 수 없어요. Job 25 구현을 완료한 에이전트로 업데이트해 주세요."
		s.render(writer, data)
		return
	}
	supportedBrowser := s.SupportedBrowser
	if supportedBrowser == nil {
		supportedBrowser = browser.Supported
	}
	installation, supported := supportedBrowser(values.BrowserBinary)
	if !supported {
		data.Error = "지원하는 전용 Chromium 브라우저를 선택해 주세요."
		s.render(writer, data)
		return
	}
	if action == "login" {
		openLogin := s.OpenLogin
		if openLogin == nil {
			openLogin = browser.OpenLogin
		}
		if err := openLogin(values.BrowserBinary, draft.ProfileDir); err != nil {
			data.Error = err.Error()
		} else {
			data.Message = "전용 브라우저를 열었습니다. 네이버 로그인을 마친 뒤 이 화면에서 연결을 완료하세요. Postpilot이 내 블로그의 글쓰기 화면을 열고 계정과 카테고리를 검증합니다."
		}
		s.render(writer, data)
		return
	}
	if strings.TrimSpace(values.DeviceCode) == "" {
		data.Error = "연결 코드를 입력해 주세요."
		s.render(writer, data)
		return
	}
	if request.FormValue("identity_confirmed") != "yes" {
		data.Error = "전용 브라우저에서 네이버 로그인을 마치고 확인란을 선택해 주세요."
		s.render(writer, data)
		return
	}
	openEditor := s.OpenEditor
	if openEditor == nil {
		openEditor = browser.OpenEditor
	}
	browserSession, err := openEditor(values.BrowserBinary, draft.ProfileDir)
	if err != nil {
		data.Error = "전용 브라우저에서 네이버 글쓰기 화면을 열지 못했어요: " + err.Error() + ". 로그인 또는 보안 확인이 필요한지 확인한 뒤 다시 시도하세요."
		s.render(writer, data)
		return
	}
	defer browserSession.Close()
	probe, err := s.ProbePublisher(request.Context(), browserSession.CDPURL)
	if err != nil || strings.TrimSpace(probe.ExecutorVersion) == "" || strings.TrimSpace(probe.Identity.BlogID) == "" {
		data.Error = "Naver 퍼블리셔 호환성 검사를 통과하지 못해 연결을 활성화하지 않았어요: " + probeError(err)
		s.render(writer, data)
		return
	}
	identity := probe.Identity
	categories := make([]*postpilotv1.PublishingCategory, 0, len(identity.Categories))
	for _, category := range identity.Categories {
		categories = append(categories, &postpilotv1.PublishingCategory{Id: category.ID, Name: category.Name})
	}
	defaultCategoryID := identity.Categories[0].ID
	pendingIndex := -1
	for index, existing := range cfg.Connections {
		if existing.DraftID != draft.ID {
			continue
		}
		if existing.Armed {
			data.Error = "이미 연결된 Mac 에이전트예요."
			s.render(writer, data)
			return
		}
		pendingIndex = index
		break
	}

	var agentID, keyAccount, agentToken string
	var leaseTTLSeconds int64
	if pendingIndex >= 0 {
		pending := cfg.Connections[pendingIndex]
		if pending.APIURL != strings.TrimRight(values.APIURL, "/") {
			data.Error = "저장된 미완료 연결과 API 주소가 달라요. 같은 주소로 다시 시도해 주세요."
			s.render(writer, data)
			return
		}
		agentID, keyAccount, leaseTTLSeconds = pending.AgentID, pending.KeychainAccount, pending.LeaseTTLSeconds
		agentToken, err = s.Keychain.Get(request.Context(), keyAccount)
		if err != nil {
			data.Error = "미완료 연결의 Keychain 토큰을 찾지 못했어요. Postpilot에서 새 연결 코드를 만들어 다시 연결해 주세요."
			s.render(writer, data)
			return
		}
	} else {
		enroll := s.Enroll
		if enroll == nil {
			enroll = postpilot.Enroll
		}
		enrollment, enrollErr := enroll(request.Context(), values.APIURL, values.DeviceCode, installation.Label)
		if enrollErr != nil {
			data.Error = "연결 코드를 사용할 수 없어요: " + enrollErr.Error()
			s.render(writer, data)
			return
		}
		agentID = enrollment.GetAgentId()
		agentToken = enrollment.GetAgentToken()
		leaseTTLSeconds = enrollment.GetLeaseTtlSeconds()
		keyAccount = "agent-" + agentID
		if err := s.Keychain.Put(request.Context(), keyAccount, agentToken); err != nil {
			data.Error = "Keychain에 토큰을 저장하지 못했어요."
			s.render(writer, data)
			return
		}
		// Persist a recoverable, explicitly unarmed local connection before the server
		// can expose it as ready. If SyncProfile commits but its response is lost, the
		// same setup submission resumes with this Keychain token instead of consuming a
		// second device code or leaving an unreachable ready agent.
		pending := config.Connection{ID: draft.ID, DraftID: draft.ID, Label: draft.Label, APIURL: draft.APIURL, AgentID: agentID, KeychainAccount: keyAccount, BrowserBinary: draft.BrowserBinary, BrowserLabel: draft.BrowserLabel, ProfileDir: draft.ProfileDir, WorkDir: draft.WorkDir, LeaseTTLSeconds: leaseTTLSeconds}
		cfg, pendingIndex, err = persistPending(s.Paths, cfg, pending)
		if err != nil {
			_ = s.Keychain.Delete(request.Context(), keyAccount)
			data.Error = "로컬 연결 상태를 저장하지 못했어요: " + err.Error()
			s.render(writer, data)
			return
		}
	}
	// Enrollment is not enough to advertise a usable Mac. Reject an incompatible
	// server lease before SyncProfile can mark compatibility_ready or activation can
	// make the daemon select this connection.
	if err := config.ValidateLeaseTTL(leaseTTLSeconds); err != nil {
		data.Error = "서버의 발행 lease 설정이 이 Mac의 heartbeat와 맞지 않아 연결을 활성화하지 않았어요: " + err.Error()
		s.render(writer, data)
		return
	}

	agent, err := s.syncProfile(request.Context(), values.APIURL, agentToken, &postpilotv1.SyncAgentProfileRequest{PlatformAccountId: identity.BlogID, PlatformAccountLabel: identity.BlogLabel, BrowserLabel: installation.Label + " · " + probe.BrowserVersion, Categories: categories, DefaultCategoryId: defaultCategoryID, DefaultVisibility: postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PUBLIC, CompatibilityReady: true, ExecutorVersion: probe.ExecutorVersion})
	if err != nil {
		data.Error = "서버에 Mac 프로필을 확인하지 못했어요. 미완료 연결은 이 Mac에 안전하게 보관했으니 같은 연결 코드로 다시 시도하세요: " + err.Error()
		s.render(writer, data)
		return
	}
	if agent.GetId() != agentID {
		data.Error = "서버가 다른 Mac 연결을 반환해 활성화하지 않았어요."
		s.render(writer, data)
		return
	}
	connection := cfg.Connections[pendingIndex]
	connection.Label = draft.Label
	connection.BrowserBinary = draft.BrowserBinary
	connection.BrowserLabel = installation.Label
	connection.PlatformAccountID = identity.BlogID
	connection.BrowserVersion = probe.BrowserVersion
	connection.CompatibilitySignature = probe.SignatureID
	connection.ExecutorVersion = probe.ExecutorVersion
	connection.ProfileDir = draft.ProfileDir
	connection.WorkDir = draft.WorkDir
	if err := activatePending(s.Paths, cfg, pendingIndex, draftIndex, connection); err != nil {
		data.Error = "서버 확인은 끝났지만 로컬 활성화 저장에 실패했어요. 미완료 연결과 토큰은 남아 있으니 같은 연결 코드로 다시 시도하세요: " + err.Error()
		s.render(writer, data)
		return
	}
	data.Message = "연결을 마쳤습니다. 실행 중인 LaunchAgent가 이 연결을 자동으로 감지하며, Mac이 다시 켜져도 자동으로 대기합니다."
	data.Values.DeviceCode = ""
	s.render(writer, data)
	s.finish()
}

func (s Server) finish() {
	if s.completed == nil {
		return
	}
	select {
	case s.completed <- struct{}{}:
	default:
	}
}

func persistPending(paths config.Paths, cfg config.File, connection config.Connection) (config.File, int, error) {
	connection.Armed = false
	for _, existing := range cfg.Connections {
		if existing.ID == connection.ID || (connection.DraftID != "" && existing.DraftID == connection.DraftID) {
			return cfg, -1, errors.New("connection draft is already bound")
		}
	}
	cfg.Connections = append(cfg.Connections, connection)
	index := len(cfg.Connections) - 1
	if err := config.Save(paths, cfg); err != nil {
		return cfg, -1, err
	}
	return cfg, index, nil
}

func activatePending(paths config.Paths, cfg config.File, index, draftIndex int, connection config.Connection) error {
	if index < 0 || index >= len(cfg.Connections) || cfg.Connections[index].ID != connection.ID {
		return errors.New("pending connection changed before activation")
	}
	if draftIndex < 0 || draftIndex >= len(cfg.Drafts) || cfg.Drafts[draftIndex].ID != connection.DraftID {
		return errors.New("connection draft changed before activation")
	}
	draft := cfg.Drafts[draftIndex]
	if connection.ProfileDir != draft.ProfileDir || connection.WorkDir != draft.WorkDir {
		return errors.New("pending connection paths conflict with its draft")
	}
	connection.Armed = true
	connection.DraftID = ""
	cfg.Connections[index] = connection
	cfg.Drafts = append(cfg.Drafts[:draftIndex], cfg.Drafts[draftIndex+1:]...)
	return config.Save(paths, cfg)
}

func (s Server) createDraft(writer http.ResponseWriter, data pageData) {
	values := data.Values
	if values.BrowserBinary == "" || strings.TrimSpace(values.Label) == "" {
		data.Error = "브라우저와 연결 이름을 입력해 주세요."
		s.render(writer, data)
		return
	}
	if err := config.ValidateAPIURL(values.APIURL); err != nil {
		data.Error = err.Error()
		s.render(writer, data)
		return
	}
	supportedBrowser := s.SupportedBrowser
	if supportedBrowser == nil {
		supportedBrowser = browser.Supported
	}
	installation, ok := supportedBrowser(values.BrowserBinary)
	if !ok {
		data.Error = "지원하는 전용 Chromium 브라우저를 선택해 주세요."
		s.render(writer, data)
		return
	}
	newID := s.NewDraftID
	if newID == nil {
		newID = randomDraftID
	}
	id, err := newID()
	if err != nil || strings.TrimSpace(id) == "" {
		data.Error = "새 연결의 로컬 식별자를 만들지 못했어요."
		s.render(writer, data)
		return
	}
	cfg, err := config.Load(s.Paths)
	if err != nil {
		data.Error = "저장된 연결을 읽지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	for _, draft := range cfg.Drafts {
		if draft.ID == id {
			data.Error = "새 연결 식별자가 기존 초안과 충돌했어요. 다시 시도해 주세요."
			s.render(writer, data)
			return
		}
	}
	for _, connection := range cfg.Connections {
		if connection.ID == id {
			data.Error = "새 연결 식별자가 기존 연결과 충돌했어요. 다시 시도해 주세요."
			s.render(writer, data)
			return
		}
	}
	profileDir, err := browser.PrepareProfile(s.Paths.Profiles, id)
	if err != nil {
		data.Error = "전용 브라우저 프로필을 준비하지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	workDir := filepath.Join(s.Paths.Jobs, id)
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		data.Error = "로컬 작업 폴더를 준비하지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	now := s.Now
	if now == nil {
		now = time.Now
	}
	cfg.Drafts = append(cfg.Drafts, config.ConnectionDraft{
		ID: id, Label: strings.TrimSpace(values.Label), APIURL: strings.TrimRight(values.APIURL, "/"),
		BrowserBinary: values.BrowserBinary, BrowserLabel: installation.Label,
		ProfileDir: profileDir, WorkDir: workDir, CreatedAt: now(),
	})
	if err := config.Save(s.Paths, cfg); err != nil {
		data.Error = "연결 초안을 저장하지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	data.Message = "새 연결 초안을 만들었습니다. 아래 미완료 연결에서 같은 전용 브라우저를 열고, 준비되면 현재 연결 코드를 입력하세요."
	s.render(writer, data)
}

func exactDraft(cfg config.File, id string) (config.ConnectionDraft, int, error) {
	if id == "" {
		return config.ConnectionDraft{}, -1, errors.New("재개할 연결 초안을 선택해 주세요.")
	}
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 16 || id != strings.ToLower(id) {
		return config.ConnectionDraft{}, -1, errors.New("연결 초안 식별자가 올바르지 않아 안전하게 열 수 없어요.")
	}
	index := -1
	for i, draft := range cfg.Drafts {
		if draft.ID != id {
			continue
		}
		if index >= 0 {
			return config.ConnectionDraft{}, -1, errors.New("같은 식별자의 연결 초안이 중복되어 안전하게 열 수 없어요.")
		}
		index = i
	}
	if index < 0 {
		return config.ConnectionDraft{}, -1, errors.New("재개할 연결 초안을 찾지 못했어요.")
	}
	return cfg.Drafts[index], index, nil
}

func randomDraftID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("create connection draft id: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (s Server) render(writer http.ResponseWriter, data pageData) {
	data.Nonce = s.nonce
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	// The POST boundary requires an exact Origin. Chromium serializes form origins as
	// `null` under `no-referrer`, so retain referrers only within this loopback origin;
	// cross-origin navigations still receive none.
	writer.Header().Set("Referrer-Policy", "same-origin")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if cfg, err := config.Load(s.Paths); err == nil {
		for _, draft := range cfg.Drafts {
			data.Drafts = append(data.Drafts, draftView{ID: draft.ID, Label: draft.Label, BrowserLabel: draft.BrowserLabel, APIURL: draft.APIURL})
		}
		for _, connection := range cfg.Connections {
			if connection.Armed {
				status := "호환성 재검사 필요"
				if connection.BrowserVersion != "" && connection.CompatibilitySignature != "" {
					status = connection.BrowserVersion + " · driver " + connection.CompatibilitySignature
				}
				data.Connections = append(data.Connections, repairConnection{
					ID: connection.ID, Label: connection.Label, BrowserLabel: connection.BrowserLabel, Status: status,
				})
			}
		}
	} else if data.Error == "" {
		data.Error = "저장된 연결 구성이 올바르지 않아 안전하게 열 수 없어요: " + err.Error()
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = setupPage.Execute(writer, data)
}

func setupNonce() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("create setup nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s Server) authorize(writer http.ResponseWriter, request *http.Request, nonce string) bool {
	if s.nonce == "" || s.host == "" || request.Host != s.host || nonce != s.nonce {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return false
	}
	expectedOrigin := "http://" + s.host
	if request.Method == http.MethodPost && request.Header.Get("Origin") != expectedOrigin {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return false
	}
	if site := request.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (s Server) repair(writer http.ResponseWriter, request *http.Request) {
	data := pageData{Browsers: browser.Discover(), Values: formValues{APIURL: config.DefaultAPIURL, Label: "내 Mac"}}
	cfg, err := config.Load(s.Paths)
	if err != nil {
		data.Error = "저장된 연결을 읽지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	connectionID := strings.TrimSpace(request.FormValue("connection_id"))
	for _, connection := range cfg.Connections {
		if connection.ID != connectionID || !connection.Armed {
			continue
		}
		openLogin := s.OpenLogin
		if openLogin == nil {
			openLogin = browser.OpenLogin
		}
		if err := openLogin(connection.BrowserBinary, connection.ProfileDir); err != nil {
			data.Error = "전용 네이버 로그인 브라우저를 열지 못했어요: " + err.Error()
		} else {
			data.Message = connection.Label + " 연결의 전용 브라우저를 열었습니다. 로그인 또는 보안 확인을 마친 뒤 Postpilot에서 같은 발행 작업을 안전하게 다시 시도하세요."
		}
		s.render(writer, data)
		return
	}
	data.Error = "복구할 활성 연결을 찾지 못했어요."
	s.render(writer, data)
}

func (s Server) reprobe(writer http.ResponseWriter, request *http.Request) {
	data := pageData{Browsers: browser.Discover(), Values: formValues{APIURL: config.DefaultAPIURL, Label: "내 Mac"}}
	cfg, err := config.Load(s.Paths)
	if err != nil {
		data.Error = "저장된 연결을 읽지 못했어요: " + err.Error()
		s.render(writer, data)
		return
	}
	connectionID := strings.TrimSpace(request.FormValue("connection_id"))
	for index := range cfg.Connections {
		connection := &cfg.Connections[index]
		if connection.ID != connectionID || !connection.Armed {
			continue
		}
		if s.ProbePublisher == nil {
			data.Error = "호환성 검사기가 없어 기존 연결을 준비 상태로 갱신하지 않았어요."
			s.render(writer, data)
			return
		}
		openEditor := s.OpenEditor
		if openEditor == nil {
			openEditor = browser.OpenEditor
		}
		session, openErr := openEditor(connection.BrowserBinary, connection.ProfileDir)
		if openErr != nil {
			data.Error = "기존 전용 프로필을 열지 못해 준비 상태를 갱신하지 않았어요: " + openErr.Error()
			s.render(writer, data)
			return
		}
		defer session.Close()
		probe, probeErr := s.ProbePublisher(request.Context(), session.CDPURL)
		if probeErr != nil {
			data.Error = "호환성 재검사를 통과하지 못해 준비 상태를 갱신하지 않았어요: " + probeErr.Error()
			s.render(writer, data)
			return
		}
		if connection.PlatformAccountID != "" && connection.PlatformAccountID != probe.Identity.BlogID {
			data.Error = "기존 연결과 다른 네이버 블로그가 선택되어 준비 상태를 갱신하지 않았어요. 같은 계정으로 다시 로그인해 주세요."
			s.render(writer, data)
			return
		}
		token, tokenErr := s.Keychain.Get(request.Context(), connection.KeychainAccount)
		if tokenErr != nil {
			data.Error = "기존 연결의 Keychain 토큰을 찾지 못해 준비 상태를 갱신하지 않았어요."
			s.render(writer, data)
			return
		}
		categories := make([]*postpilotv1.PublishingCategory, 0, len(probe.Identity.Categories))
		for _, category := range probe.Identity.Categories {
			categories = append(categories, &postpilotv1.PublishingCategory{Id: category.ID, Name: category.Name})
		}
		if len(categories) == 0 {
			data.Error = "네이버 카테고리를 확인하지 못해 준비 상태를 갱신하지 않았어요."
			s.render(writer, data)
			return
		}
		agent, syncErr := s.syncProfile(request.Context(), connection.APIURL, token, &postpilotv1.SyncAgentProfileRequest{PlatformAccountId: probe.Identity.BlogID, PlatformAccountLabel: probe.Identity.BlogLabel, BrowserLabel: connection.BrowserLabel + " · " + probe.BrowserVersion, Categories: categories, DefaultCategoryId: categories[0].Id, DefaultVisibility: postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PUBLIC, CompatibilityReady: true, ExecutorVersion: probe.ExecutorVersion})
		if syncErr != nil || agent.GetId() != connection.AgentID || (agent.GetPlatformAccountId() != "" && agent.GetPlatformAccountId() != probe.Identity.BlogID) {
			data.Error = "서버가 기존 연결과 같은 에이전트·네이버 계정을 확인하지 않아 준비 상태를 갱신하지 않았어요."
			s.render(writer, data)
			return
		}
		connection.PlatformAccountID = probe.Identity.BlogID
		connection.BrowserVersion = probe.BrowserVersion
		connection.CompatibilitySignature = probe.SignatureID
		connection.ExecutorVersion = probe.ExecutorVersion
		if err := config.Save(s.Paths, cfg); err != nil {
			data.Error = "재검사 결과를 안전하게 저장하지 못했어요: " + err.Error()
		} else {
			data.Message = connection.Label + " 연결의 계정·카테고리·브라우저·드라이버 호환성을 다시 확인했습니다."
		}
		s.render(writer, data)
		return
	}
	data.Error = "재검사할 활성 연결을 찾지 못했어요."
	s.render(writer, data)
}

func (s Server) syncProfile(ctx context.Context, apiURL, token string, request *postpilotv1.SyncAgentProfileRequest) (*postpilotv1.PublishingAgent, error) {
	if s.SyncProfile != nil {
		return s.SyncProfile(ctx, apiURL, token, request)
	}
	return postpilot.New(apiURL, token).SyncProfile(ctx, request)
}

func probeError(err error) string {
	if err == nil {
		return "검사 결과가 완전하지 않습니다"
	}
	return err.Error()
}
