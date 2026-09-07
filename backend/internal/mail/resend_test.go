package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/auth"
)

func TestResendSendsTheDocumentedRequest(t *testing.T) {
	var got struct {
		method, path, authorization, contentType, userAgent string
		body                                                map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		got.authorization = r.Header.Get("Authorization")
		got.contentType = r.Header.Get("Content-Type")
		got.userAgent = r.Header.Get("User-Agent")
		if err := json.NewDecoder(r.Body).Decode(&got.body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"email-id"}`))
	}))
	defer server.Close()

	adapter := NewResend("re_secret", "PostPilot <mail@example.com>", server.Client())
	adapter.endpoint = server.URL + "/emails"
	want := auth.Mail{To: "alice@example.com", Subject: "Subject", Text: "plain body"}
	if err := adapter.Send(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/emails" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.authorization != "Bearer re_secret" || got.contentType != "application/json" || got.userAgent == "" {
		t.Errorf("headers: authorization=%q content-type=%q user-agent=%q", got.authorization, got.contentType, got.userAgent)
	}
	if got.body["from"] != "PostPilot <mail@example.com>" || got.body["subject"] != want.Subject || got.body["text"] != want.Text {
		t.Errorf("body = %#v", got.body)
	}
	to, ok := got.body["to"].([]any)
	if !ok || len(to) != 1 || to[0] != want.To {
		t.Errorf("to = %#v", got.body["to"])
	}
}

func TestResendMapsRecipientValidationAndOtherFailures(t *testing.T) {
	for name, test := range map[string]struct {
		status int
		body   string
		check  func(error) bool
	}{
		"recipient": {
			status: http.StatusUnprocessableEntity,
			body:   `{"name":"validation_error","message":"The 'to' field is invalid."}`,
			check:  func(err error) bool { return errors.Is(err, auth.ErrRecipientRejected) },
		},
		"provider": {
			status: http.StatusInternalServerError,
			body:   `{"name":"application_error","message":"try later"}`,
			check: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "500 Internal Server Error")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			adapter := NewResend("key", "mail@example.com", server.Client())
			adapter.endpoint = server.URL
			if err := adapter.Send(context.Background(), auth.Mail{To: "alice@example.com"}); !test.check(err) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLogMailerWritesEveryFieldAtInfo(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	mail := auth.Mail{To: "alice@example.com", Subject: "subject", Text: "body"}
	if err := NewLog().Send(context.Background(), mail); err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	for _, value := range []string{mail.To, mail.Subject, mail.Text, "level=INFO"} {
		if !strings.Contains(logged, value) {
			t.Fatalf("log %q does not contain %q", logged, value)
		}
	}
}
