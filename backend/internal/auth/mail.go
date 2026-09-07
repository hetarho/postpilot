package auth

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

func verificationMail(email, link string) Mail {
	return Mail{
		To:      email,
		Subject: "PostPilot 이메일 인증 / Verify your email",
		Text: strings.Join([]string{
			"PostPilot 이메일 인증",
			"아래 링크를 열어 이메일 주소를 인증하세요.",
			link,
			"",
			"Verify your PostPilot email",
			"Open the link above to verify your email address.",
		}, "\n"),
	}
}

func existingAccountMail(email string) Mail {
	return Mail{
		To:      email,
		Subject: "PostPilot 계정 안내 / Account notice",
		Text: strings.Join([]string{
			"PostPilot 계정 안내",
			"이 이메일 주소로 이미 계정이 등록되어 있습니다.",
			"본인이 요청하지 않았다면 이 메일을 무시하세요.",
			"",
			"PostPilot account notice",
			"An account already exists for this email address.",
			"If you did not make this request, you can ignore this email.",
		}, "\n"),
	}
}

func passwordResetMail(email, link string) Mail {
	return Mail{
		To:      email,
		Subject: "PostPilot 비밀번호 재설정 / Reset your password",
		Text: strings.Join([]string{
			"PostPilot 비밀번호 재설정",
			"아래 링크를 열어 비밀번호를 재설정하세요.",
			link,
			"",
			"Reset your PostPilot password",
			"Open the link above to reset your password.",
		}, "\n"),
	}
}

func lockNoticeMail(email string, until time.Time) Mail {
	instant := until.UTC().Format(time.RFC3339)
	return Mail{
		To:      email,
		Subject: "PostPilot 로그인 잠금 안내 / Sign-in lock notice",
		Text: strings.Join([]string{
			"PostPilot 로그인 잠금 안내",
			"연속된 로그인 실패로 계정이 다음 시각까지 잠겼습니다: " + instant,
			"본인이 시도하지 않았다면 비밀번호를 재설정하세요.",
			"",
			"PostPilot sign-in lock notice",
			"Repeated sign-in failures locked the account until: " + instant,
			"If this was not you, reset your password.",
		}, "\n"),
	}
}

func (s *Service) verificationLink(raw string) (string, error) {
	return buildAuthLink(s.webOrigin, "/verify-email", raw)
}

func (s *Service) passwordResetLink(raw string) (string, error) {
	return buildAuthLink(s.webOrigin, "/reset-password", raw)
}

func buildAuthLink(origin, path, raw string) (string, error) {
	if strings.TrimSpace(origin) == "" {
		return "", errors.New("web origin is required")
	}
	if raw == "" {
		return "", errors.New("auth link token is required")
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("web origin must be an absolute URL")
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = url.Values{"token": []string{raw}}.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}
