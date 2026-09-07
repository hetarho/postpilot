package launchd

import (
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const Label = "com.postpilot.publishing-agent"

// launchctlBootout unloads one agent. It is a package variable so a test can prove the
// unload is requested without stopping a genuinely installed agent on the developer's Mac.
var launchctlBootout = func(domainTarget string) {
	_ = exec.Command("/bin/launchctl", "bootout", domainTarget).Run()
}

func domainTarget() string { return "gui/" + strconv.Itoa(os.Getuid()) + "/" + Label }

func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

func Install(binary, logDir string) error {
	if !filepath.IsAbs(binary) || !filepath.IsAbs(logDir) {
		return errors.New("LaunchAgent binary and log directory must be absolute")
	}
	path, err := PlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(logDir, 0o700); err != nil {
		return err
	}
	if err := writePlist(path, binary, logDir); err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	launchctlBootout(domainTarget())
	return exec.Command("/bin/launchctl", "bootstrap", domain, path).Run()
}

func writePlist(path, binary, logDir string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>run</string></array>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
<key>ProcessType</key><string>Background</string>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, Label, html.EscapeString(binary), html.EscapeString(filepath.Join(logDir, "agent.log")), html.EscapeString(filepath.Join(logDir, "agent-error.log")))
	return os.WriteFile(path, []byte(plist), 0o600)
}

func Uninstall() error {
	path, err := PlistPath()
	if err != nil {
		return err
	}
	launchctlBootout(domainTarget())
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
