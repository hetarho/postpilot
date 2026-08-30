package credentials

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

const service = "com.postpilot.publishing-agent"

type Store interface {
	Put(context.Context, string, string) error
	Get(context.Context, string) (string, error)
	Delete(context.Context, string) error
}

type Keychain struct{}

func (Keychain) Put(ctx context.Context, account, token string) error {
	if account == "" || token == "" {
		return errors.New("empty keychain account or token")
	}
	// Apple's security CLI explicitly marks `-w <password>` as insecure because the
	// secret appears in argv. A trailing -w prompts on stdin instead.
	command := exec.CommandContext(ctx, "/usr/bin/security", "add-generic-password", "-a", account, "-s", service, "-U", "-w")
	command.Stdin = strings.NewReader(token + "\n")
	return command.Run()
}

func (Keychain) Get(ctx context.Context, account string) (string, error) {
	out, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-a", account, "-s", service, "-w").Output()
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", errors.New("empty token in keychain")
	}
	return token, nil
}

func (Keychain) Delete(ctx context.Context, account string) error {
	return exec.CommandContext(ctx, "/usr/bin/security", "delete-generic-password", "-a", account, "-s", service).Run()
}
