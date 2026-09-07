package auth

import (
	"strings"
	"testing"
	"time"
)

func TestAuthLinkURLsUseTheConfiguredWebOrigin(t *testing.T) {
	svc := &Service{}
	if _, err := svc.verificationLink("raw"); err == nil {
		t.Fatal("verification link accepted an empty web origin")
	}
	svc.SetWebOrigin("https://postpilot.example.com/")
	verification, err := svc.verificationLink("raw+/=")
	if err != nil || verification != "https://postpilot.example.com/verify-email?token=raw%2B%2F%3D" {
		t.Fatalf("verification link = %q, %v", verification, err)
	}
	reset, err := svc.passwordResetLink("raw")
	if err != nil || reset != "https://postpilot.example.com/reset-password?token=raw" {
		t.Fatalf("reset link = %q, %v", reset, err)
	}
}

func TestTransactionalMailBuildersAreBilingualPlainText(t *testing.T) {
	raw, hash, err := NewLinkToken()
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{}
	svc.SetWebOrigin("https://postpilot.example.com")
	verifyLink, err := svc.verificationLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	resetLink, err := svc.passwordResetLink(raw)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		mail          Mail
		koreanMarker  string
		englishMarker string
		link          string
	}{
		"verification": {
			mail:         verificationMail("alice@example.com", verifyLink),
			koreanMarker: "이메일 인증", englishMarker: "Verify your PostPilot email", link: verifyLink,
		},
		"existing account": {
			mail:         existingAccountMail("alice@example.com"),
			koreanMarker: "계정 안내", englishMarker: "PostPilot account notice",
		},
		"password reset": {
			mail:         passwordResetMail("alice@example.com", resetLink),
			koreanMarker: "비밀번호 재설정", englishMarker: "Reset your PostPilot password", link: resetLink,
		},
		"lock notice": {
			mail:         lockNoticeMail("alice@example.com", time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)),
			koreanMarker: "로그인 잠금 안내", englishMarker: "PostPilot sign-in lock notice",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mail.To != "alice@example.com" || test.mail.Subject == "" || test.mail.Text == "" {
				t.Fatalf("mail = %+v", test.mail)
			}
			if ko, en := strings.Index(test.mail.Text, test.koreanMarker), strings.Index(test.mail.Text, test.englishMarker); ko < 0 || en < 0 || ko >= en {
				t.Fatalf("mail is not Korean-first then English:\n%s", test.mail.Text)
			}
			if strings.Count(test.mail.Text, "\n")+1 > 15 {
				t.Fatalf("mail has more than 15 lines:\n%s", test.mail.Text)
			}
			if strings.ContainsAny(test.mail.Text, "<>") {
				t.Fatalf("mail contains generated HTML:\n%s", test.mail.Text)
			}
			if strings.Contains(test.mail.Text, hash) {
				t.Fatal("mail exposed the stored token hash")
			}
			if test.link != "" {
				if !strings.Contains(test.mail.Text, test.link) || strings.Count(test.mail.Text, raw) != 1 {
					t.Fatalf("raw token is not present exactly once inside its link:\n%s", test.mail.Text)
				}
			} else if strings.Contains(test.mail.Text, raw) {
				t.Fatal("a mail without a link contains the raw token")
			}
		})
	}
}
