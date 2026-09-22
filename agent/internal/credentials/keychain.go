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
	// security's interactive `-w` (no value) reads the password from the controlling
	// TTY, not stdin, so a piped token is ignored and setup hangs on a prompt (and a
	// TTY-less LaunchAgent could never store one). Pass the token as the -w value; it is
	// briefly visible in this process's argv, an accepted tradeoff on a local single Mac.
	command := exec.CommandContext(ctx, "/usr/bin/security", "add-generic-password", "-a", account, "-s", service, "-U", "-w", token)
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
	if account == "" {
		return errors.New("empty keychain account")
	}
	output, err := exec.CommandContext(ctx, "/usr/bin/security", "delete-generic-password", "-a", account, "-s", service).CombinedOutput()
	if err == nil {
		return nil
	}
	// macOS security exits 44 when the requested item does not exist. A repeated
	// retirement must be safe, so absence is success without first retrieving the
	// secret. Keep the text check for older security builds that return exit 1.
	var exitErr *exec.ExitError
	if (errors.As(err, &exitErr) && exitErr.ExitCode() == 44) || strings.Contains(strings.ToLower(string(output)), "could not be found") {
		return nil
	}
	return err
}
