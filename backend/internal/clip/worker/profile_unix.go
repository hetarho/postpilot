//go:build unix

package worker

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"golang.org/x/sys/unix"
)

const profileFile = ".worker-profile.json"

type rootOwner struct {
	PID        int
	Generation string
}

// ProfileBinding identifies the deployment configuration and the executable,
// tools and assets whose capabilities the owning process actually validated.
type ProfileBinding struct {
	WorkerID, Configuration, RuntimeIdentity string
}

type activeProfile struct {
	Version int
	Owner   rootOwner
	Binding ProfileBinding
	Profile clip.MediaWorkerProfile
}

func startRootOwner(root string, lock *os.File) error {
	// Invalidate the previous process before any startup capability checks. A
	// successor holding the kernel lock alone must never inherit readiness.
	if err := os.Remove(filepath.Join(root, profileFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	raw, err := json.Marshal(rootOwner{PID: os.Getpid(), Generation: hex.EncodeToString(nonce[:])})
	if err != nil {
		return err
	}
	if err = lock.Truncate(0); err != nil {
		return err
	}
	_, err = lock.WriteAt(raw, 0)
	return err
}

func readPrivateJSON(path string, limit int64, value any) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), "worker readiness")
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return errors.New("invalid worker readiness file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return errors.New("invalid worker readiness file")
	}
	return json.Unmarshal(raw, value)
}

func readRootOwner(root string) (rootOwner, error) {
	var owner rootOwner
	err := readPrivateJSON(filepath.Join(root, ".worker-lock"), 1024, &owner)
	if err != nil || owner.PID <= 0 || len(owner.Generation) != 32 {
		return rootOwner{}, errors.New("media worker owner is not ready")
	}
	return owner, nil
}

// PublishProfile is called by the lock owner only after real capability
// validation succeeds. The private atomic snapshot contains no credentials.
func PublishProfile(root string, binding ProfileBinding, profile clip.MediaWorkerProfile) error {
	if profile.Operation == "" {
		profile.Operation = clip.MediaRender
	}
	if profile.Validate() != nil || !profile.Compatible() || profile.WorkerID != binding.WorkerID {
		return errors.New("invalid validated media worker profile")
	}
	if err := CheckWorkRootActive(root); err != nil {
		return err
	}
	owner, err := readRootOwner(root)
	if err != nil || owner.PID != os.Getpid() {
		return errors.New("media worker does not own this profile")
	}
	raw, err := json.Marshal(activeProfile{Version: 1, Owner: owner, Binding: binding, Profile: profile})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".worker-profile-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(root, profileFile))
}

// ActiveProfile reads the current owner's already validated profile. An active
// lock, owner generation, runtime and deployment must all still agree.
func ActiveProfile(root string, binding ProfileBinding) (clip.MediaWorkerProfile, error) {
	if err := CheckWorkRootActive(root); err != nil {
		return clip.MediaWorkerProfile{}, err
	}
	owner, err := readRootOwner(root)
	if err != nil {
		return clip.MediaWorkerProfile{}, err
	}
	var cached activeProfile
	if err = readPrivateJSON(filepath.Join(root, profileFile), 2*clip.MediaManifestMaxBytes+4096, &cached); err != nil {
		return clip.MediaWorkerProfile{}, errors.New("media worker validated profile is not ready")
	}
	if cached.Version != 1 || cached.Owner != owner || cached.Binding != binding || cached.Profile.WorkerID != binding.WorkerID || cached.Profile.Validate() != nil || !cached.Profile.Compatible() {
		return clip.MediaWorkerProfile{}, errors.New("media worker validated profile does not match this runtime")
	}
	// A release and successor acquisition between the first read and this check
	// cannot turn a previous process's snapshot into the successor's readiness.
	again, err := readRootOwner(root)
	if err != nil || again != owner {
		return clip.MediaWorkerProfile{}, errors.New("media worker owner changed")
	}
	if err = CheckWorkRootActive(root); err != nil {
		return clip.MediaWorkerProfile{}, err
	}
	return cached.Profile, nil
}

// RuntimeIdentity streams the configured artifacts without loading font faces
// or launching tool subprocesses. Changed image/tool/assets invalidate a cache.
func RuntimeIdentity(paths []string) (string, error) {
	paths = slices.Clone(paths)
	slices.Sort(paths)
	hash := sha256.New()
	buffer := make([]byte, 32<<10)
	read := func(path string) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("invalid worker runtime artifact")
		}
		content := sha256.New()
		if _, err = io.CopyBuffer(content, f, buffer); err != nil {
			return err
		}
		_, _ = io.WriteString(hash, path+"\x00"+hex.EncodeToString(content.Sum(nil))+"\x00")
		return nil
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			if err = read(path); err != nil {
				return "", err
			}
			continue
		}
		if err = filepath.WalkDir(path, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			return read(name)
		}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
