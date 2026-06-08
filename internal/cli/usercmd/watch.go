package usercmd

import (
	"archive/tar"
	"context"
	"crypto/md5" // #nosec G501 -- md5 used for change detection only, not cryptography
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

const (
	watchStateFile    = ".epinio-patch-state"
	watchConfigFile   = ".epinio-sync.yaml"
	watchPollInterval = 500 * time.Millisecond
)

type syncConfig struct {
	BuildCmd   string `yaml:"build_cmd"`
	Binary     string `yaml:"binary"`
	FilesDest  string `yaml:"files_dest"`
	// BinaryDest overrides /epinio-sync/app in the pod. Note: the supervisor
	// wrapper injected at startup also checks /epinio-sync/app, so changing this
	// without a matching change to the startup PATCH will break the binary swap.
	BinaryDest string `yaml:"binary_dest"`
}

// fileHashes maps relative file paths to their md5 hex digest.
type fileHashes map[string]string

// AppWatch watches the source directory at path for changes and syncs them into
// the running pod. On first run (no state file) it does a full buildpack push
// to warm the cache and set up the supervisor wrapper. Subsequent runs do
// incremental file or binary syncs via the /sync endpoint, which requires no
// kubectl.
//
// namespace overrides c.Settings.Namespace when non-empty.
func (c *EpinioClient) AppWatch(
	ctx context.Context,
	appName,
	namespace,
	path string,
) error {
	if namespace == "" {
		namespace = c.Settings.Namespace
	}
	if namespace == "" {
		return fmt.Errorf(
			"namespace is required: use --namespace or run 'epinio target <namespace>'",
		)
	}
	appRef := models.NewAppRef(appName, namespace)
	log := c.Log.WithName("AppWatch")

	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, "resolving source path")
	}

	cfg, err := loadSyncConfig(absPath)
	if err != nil {
		return errors.Wrap(err, "reading .epinio-sync.yaml")
	}

	stateFile := filepath.Join(absPath, watchStateFile)
	log.Info("watching", "app", appName, "namespace", namespace, "path", absPath)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		state, stateErr := loadHashes(stateFile)
		if stateErr != nil || len(state) == 0 {
			c.ui.Normal().Msg("Startup: running full buildpack push...")

			watchStartError := c.watchStartup(
				ctx,
				appRef,
				absPath,
				stateFile,
				cfg,
			)
			if watchStartError != nil {
				return watchStartError
			}
			continue
		}

		current, hashError := hashDir(absPath)
		if hashError != nil {
			return errors.Wrap(hashError, "hashing source directory")
		}

		changed, deleted := diffHashes(state, current)
		if len(deleted) > 0 {
			c.ui.Normal().Msg(
				"Deleted files detected, clearing state for full push on next cycle...",
			)

			rmError := os.Remove(stateFile)
			if rmError != nil && !os.IsNotExist(rmError) {
				return errors.Wrap(rmError, "removing state file")
			}
			continue
		}

		if len(changed) == 0 {
			time.Sleep(watchPollInterval)
			continue
		}

		c.ui.Normal().Msgf("Detected %d changed file(s), syncing...", len(changed))
		syncError := c.watchSync(
			ctx,
			appRef,
			absPath,
			changed,
			stateFile,
			current,
			cfg,
		)
		if syncError != nil {
			return syncError
		}
	}
}

func (c *EpinioClient) watchStartup(
	ctx context.Context,
	appRef models.AppRef,
	path, stateFile string,
	cfg syncConfig,
) error {
	log := c.Log.WithName("watchStartup")

	ignorePatterns := []string{watchStateFile, watchConfigFile, ".git", "bin"}
	tmpDir, archive, err := helpers.Tar(path, ignorePatterns)
	if err != nil {
		return errors.Wrap(err, "creating source archive")
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			log.Error(rmErr, "removing temp dir")
		}
	}()

	file, err := os.Open(archive)
	if err != nil {
		return errors.Wrap(err, "opening source archive")
	}
	defer func() { _ = file.Close() }()

	c.ui.Normal().Msg("Uploading source...")
	unixTime := time.Now()
	stageResp, err := c.API.AppSourcePatch(appRef.Namespace, appRef.Name, file)
	if err != nil {
		return errors.Wrap(err, "uploading source patch")
	}
	c.ui.Normal().Msgf(
		"Uploaded in %dms,  waiting for build...",
		time.Since(unixTime).Milliseconds(),
	)

	unixTime = time.Now()
	err = stagingWait(log, c.API, appRef.Namespace, stageResp.Stage.ID)
	if err != nil {
		return errors.Wrap(err, "staging failed")
	}
	c.ui.Normal().Msgf(
		"Build done in %dms, waiting for pod...",
		time.Since(unixTime).Milliseconds(),
	)

	unixTime = time.Now()
	if err := c.waitAppRunning(ctx, appRef); err != nil {
		return errors.Wrap(err, "waiting for app to become ready")
	}
	c.ui.Normal().Msgf(
		"Pod ready in %dms. Watching for changes...",
		time.Since(unixTime).Milliseconds(),
	)

	hashes, err := hashDir(path)
	if err != nil {
		return errors.Wrap(err, "computing initial file hashes")
	}
	return saveHashes(stateFile, hashes)
}

func (c *EpinioClient) watchSync(
	ctx context.Context,
	appRef models.AppRef,
	path string,
	changed []string,
	stateFile string,
	current fileHashes,
	cfg syncConfig,
) error {
	mode := "files"
	tarBase := path
	var tarPaths []string

	dest := ""
	binaryName := ""
	if cfg.BuildCmd != "" && cfg.Binary != "" {
		mode = "binary"
		if cfg.BinaryDest != "" {
			dest = cfg.BinaryDest
		}
		c.ui.Normal().Msg("Building locally...")
		t := time.Now()
		if err := runBuildCmd(cfg.BuildCmd, path); err != nil {
			return errors.Wrap(err, "local build failed")
		}
		c.ui.Normal().Msgf("Built in %dms", time.Since(t).Milliseconds())

		binaryAbs := cfg.Binary
		if !filepath.IsAbs(cfg.Binary) {
			binaryAbs = filepath.Join(path, cfg.Binary)
		}
		tarBase = filepath.Dir(binaryAbs)
		binaryName = filepath.Base(binaryAbs)
		tarPaths = []string{binaryName}
	} else {
		if cfg.FilesDest != "" {
			dest = cfg.FilesDest
		}
		tarPaths = changed
	}

	tmpFile, err := os.CreateTemp("", "epinio-sync-*.tar")
	if err != nil {
		return errors.Wrap(err, "creating temp tar file")
	}
	tmpName := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpName) }()

	if err := createSyncTar(tmpName, tarBase, tarPaths); err != nil {
		return errors.Wrap(err, "creating sync tar")
	}

	f, err := os.Open(tmpName)
	if err != nil {
		return errors.Wrap(err, "opening sync tar")
	}
	defer func() { _ = f.Close() }()

	t := time.Now()
	if _, apiErr := c.API.AppSync(appRef.Namespace, appRef.Name, f, mode, dest, binaryName); apiErr != nil {
		errStr := apiErr.Error()
		if strings.Contains(errStr, "no ready pod") || strings.Contains(errStr, "503") {
			c.ui.Normal().Msg("No ready pod -- clearing state for full push on next cycle...")
			_ = os.Remove(stateFile)
			return nil
		}
		return errors.Wrap(apiErr, "syncing to pod")
	}
	c.ui.Normal().Msgf("Synced in %dms (via API)", time.Since(t).Milliseconds())

	return saveHashes(stateFile, current)
}

// waitAppRunning polls the /running endpoint until the app is up or ctx is cancelled.
func (c *EpinioClient) waitAppRunning(ctx context.Context, appRef models.AppRef) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := c.API.AppRunning(appRef); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// loadSyncConfig reads .epinio-sync.yaml from dir. Missing file is not an error.
func loadSyncConfig(dir string) (syncConfig, error) {
	var cfg syncConfig
	data, err := os.ReadFile(filepath.Join(dir, watchConfigFile))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, yaml.Unmarshal(data, &cfg)
}

// hashDir md5-hashes every non-excluded file under dir, returning a map of
// relative path -> hex digest. Exclusions come from .gitignore and .epinioignore
// (the same files the push command respects), plus the watch-specific state files.
func hashDir(dir string) (fileHashes, error) {
	hashes := make(fileHashes)

	// Seed the matcher with watch-internal files the user shouldn't need to list
	// themselves, then let LoadIgnoreMatcher layer on .epinioignore patterns.
	gitignorePatterns, _ := readIgnoreFile(filepath.Join(dir, ".gitignore"))
	basePatterns := append(gitignorePatterns, watchStateFile, watchConfigFile) //nolint:gocritic

	matcher, err := helpers.LoadIgnoreMatcher(dir, basePatterns)
	if err != nil {
		return nil, fmt.Errorf("loading ignore patterns: %w", err)
	}

	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Always skip .git — not a user-visible ignore, just implementation noise.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if matcher.ShouldIgnore(dir, p, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		digest, err := md5File(p)
		if err != nil {
			return fmt.Errorf("hashing %s: %w", rel, err)
		}
		hashes[rel] = digest
		return nil
	})
	return hashes, err
}

// readIgnoreFile reads pattern lines from a gitignore-style file, stripping
// blank lines and comments. Missing file is silently ignored.
func readIgnoreFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, nil
}

func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New() // #nosec G401
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func loadHashes(path string) (fileHashes, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var h fileHashes
	return h, json.Unmarshal(data, &h)
}

func saveHashes(path string, h fileHashes) error {
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// diffHashes returns the relative paths of changed/new files and deleted files.
func diffHashes(old, current fileHashes) (changed, deleted []string) {
	for path, digest := range current {
		if old[path] != digest {
			changed = append(changed, path)
		}
	}
	for path := range old {
		if _, ok := current[path]; !ok {
			deleted = append(deleted, path)
		}
	}
	return changed, deleted
}

// runBuildCmd runs the build command via sh in the given working directory.
func runBuildCmd(buildCmd, dir string) error {
	cmd := exec.Command("sh", "-c", buildCmd) // #nosec G204
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createSyncTar creates a plain (non-gzip) tar at destPath containing the listed
// files read from baseDir. Files are stored with their basenames to match what the
// server-side exec command expects.
func createSyncTar(destPath, baseDir string, files []string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	tw := tar.NewWriter(out)
	defer func() { _ = tw.Close() }()

	for _, name := range files {
		srcPath := filepath.Join(baseDir, name)
		info, err := os.Stat(srcPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", name, err)
		}

		hdr := &tar.Header{
			Name:    name,
			Mode:    int64(info.Mode()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("writing tar header for %s: %w", name, err)
		}

		f, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("opening %s: %w", name, err)
		}
		_, copyErr := io.Copy(tw, f)
		_ = f.Close()
		if copyErr != nil {
			return fmt.Errorf("writing %s to tar: %w", name, copyErr)
		}
	}
	return nil
}

