//go:build unix

package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func readyProfile() clip.MediaWorkerProfile {
	return clip.MediaWorkerProfile{WorkerID: "worker", Operation: clip.MediaRender, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: clip.MediaCPUProfile, RuntimeManifest: `{"validated":true}`}
}

func TestActiveProfileRequiresCurrentValidatedOwner(t *testing.T) {
	root := t.TempDir()
	binding := ProfileBinding{WorkerID: "worker", Configuration: "configuration", RuntimeIdentity: "image-assets"}
	if _, err := ActiveProfile(root, binding); err == nil {
		t.Fatal("stopped root became ready")
	}
	release, err := LockWorkRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err = ActiveProfile(root, binding); err == nil {
		t.Fatal("startup lock bypassed capability validation")
	}
	if err = PublishProfile(root, binding, readyProfile()); err != nil {
		t.Fatal(err)
	}
	got, err := ActiveProfile(root, binding)
	if err != nil || got != readyProfile() {
		t.Fatal(got, err)
	}
	old, err := os.ReadFile(filepath.Join(root, profileFile))
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err = ActiveProfile(root, binding); err == nil {
		t.Fatal("released owner retained readiness")
	}
	release, err = LockWorkRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err = os.Stat(filepath.Join(root, profileFile)); !os.IsNotExist(err) {
		t.Fatal("startup retained previous cache", err)
	}
	if err = os.WriteFile(filepath.Join(root, profileFile), old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ActiveProfile(root, binding); err == nil {
		t.Fatal("old generation reused after restart with same PID")
	}
	if err = PublishProfile(root, binding, readyProfile()); err != nil {
		t.Fatal(err)
	}
	if _, err = ActiveProfile(root, binding); err != nil {
		t.Fatal(err)
	}
}

func TestActiveProfileRejectsDeploymentRuntimeAndCacheChanges(t *testing.T) {
	for _, change := range []string{"worker", "configuration", "image", "role", "truncated", "symlink", "oversized"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			binding := ProfileBinding{WorkerID: "worker", Configuration: "configuration", RuntimeIdentity: "image-assets"}
			release, err := LockWorkRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if err = PublishProfile(root, binding, readyProfile()); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, profileFile)
			switch change {
			case "worker":
				binding.WorkerID = "another-worker"
			case "configuration":
				binding.Configuration = "changed-configuration"
			case "image":
				binding.RuntimeIdentity = "changed-image-or-assets"
			case "role":
				binding.Configuration = "analysis-verification"
			case "truncated":
				err = os.WriteFile(path, []byte("{"), 0600)
			case "symlink":
				raw, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				elsewhere := filepath.Join(t.TempDir(), "profile")
				if e = os.WriteFile(elsewhere, raw, 0600); e != nil {
					t.Fatal(e)
				}
				if e = os.Remove(path); e != nil {
					t.Fatal(e)
				}
				err = os.Symlink(elsewhere, path)
			case "oversized":
				err = os.WriteFile(path, []byte(strings.Repeat("x", 2*clip.MediaManifestMaxBytes+8192)), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ActiveProfile(root, binding); err == nil {
				t.Fatal("changed readiness was accepted")
			}
		})
	}
}

func TestRuntimeIdentityBindsExecutableToolsAndAssets(t *testing.T) {
	root := t.TempDir()
	files := []string{filepath.Join(root, "worker"), filepath.Join(root, "ffmpeg"), filepath.Join(root, "font")}
	for _, path := range files {
		if err := os.WriteFile(path, []byte(path), 0600); err != nil {
			t.Fatal(err)
		}
	}
	initial, err := RuntimeIdentity(files)
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := RuntimeIdentity([]string{files[2], files[0], files[1]})
	if err != nil || reversed != initial {
		t.Fatal("artifact order changed identity", err)
	}
	for _, path := range files {
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, []byte("changed runtime artifact"), 0600); err != nil {
			t.Fatal(err)
		}
		changed, err := RuntimeIdentity(files)
		if err != nil || changed == initial {
			t.Fatal("artifact change retained cached identity", path, err)
		}
		if err = os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
