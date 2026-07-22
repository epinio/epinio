package usercmd

import (
	"archive/tar"
	"context"
	"crypto/md5" // #nosec G501 -- change detection only, not cryptography
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
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
	BuildCmd  string `yaml:"build_cmd"`
	Binary    string `yaml:"binary"`
	FilesDest string `yaml:"files_dest"`
	// BinaryDest overrides /epinio-sync/app in the pod. Note: the supervisor
	// wrapper injected at startup also checks /epinio-sync/app, so changing
	// this without a matching change to the startup PATCH will break the
	// binary swap.
	BinaryDest string `yaml:"binary_dest"`
	// ProcessCmd is the command the supervisor falls back to when no dev
	// binary is present in /epinio-sync/. By default the supervisor
	// discovers the entrypoint from /cnb/process ("web" when present,
	// otherwise the first process symlink). Set this for non-CNB images,
	// e.g. "/app/bin/start".
	ProcessCmd string `yaml:"process_cmd"`
}

// fileHashes maps relative file paths to their md5 hex digest.
type fileHashes map[string]string

// AppWatch watches the source directory at path for changes and syncs them
// into the running pod. On startup it always does a full buildpack push to
// warm the cache and (re)install the supervisor wrapper, then does incremental
// file or binary syncs via the /sync endpoint, which requires no kubectl. The
// startup push is unconditional because a prior `epinio push` (or any normal
// redeploy) recreates the pod without the supervisor; a stale state file must
// not let watch skip straight to syncing against a pod that cannot reload.
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
			"namespace is required: use --namespace" +
				" or run 'epinio target <namespace>'",
		)
	}
	appRef := models.NewAppRef(appName, namespace)
	log := c.Log.WithName("AppWatch")

	absPath, resolvePathError := filepath.Abs(path)
	if resolvePathError != nil {
		return errors.Wrap(resolvePathError, "resolving source path")
	}

	cfg, loadConfigError := loadSyncConfig(absPath)
	if loadConfigError != nil {
		return errors.Wrap(loadConfigError, "reading .epinio-sync.yaml")
	}

	stateFile := filepath.Join(absPath, watchStateFile)
	log.Info(
		"watching",
		"app", appName,
		"namespace", namespace,
		"path", absPath,
	)

	// Always start from a clean slate: remove any state file left by a prior
	// watch session so the first loop iteration runs watchStartup, which
	// reinstalls the supervisor. Without this, a stale state file makes watch
	// skip startup and sync against a pod that may have been redeployed
	// (e.g. by `epinio push`) without the supervisor.
	removeStateError := os.Remove(stateFile)
	if removeStateError != nil && !os.IsNotExist(removeStateError) {
		return errors.Wrap(removeStateError, "clearing stale watch state")
	}

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
				"Deleted files detected, " +
					"clearing state for full push on next cycle...",
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

		c.ui.Normal().Msgf(
			"Detected %d changed file(s), syncing...",
			len(changed),
		)
		syncError := c.watchSync(
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
	tmpDir, archive, createArchiveError := helpers.Tar(path, ignorePatterns)
	if createArchiveError != nil {
		return errors.Wrap(createArchiveError, "creating source archive")
	}
	defer func() {
		rmError := os.RemoveAll(tmpDir)
		if rmError != nil {
			log.Error(rmError, "removing temp dir")
		}
	}()

	file, openArchiveError := os.Open(archive)
	if openArchiveError != nil {
		return errors.Wrap(openArchiveError, "opening source archive")
	}
	defer func() {
		closeError := file.Close()
		if closeError != nil {
			log.Error(closeError, "closing source archive")
		}
	}()

	c.ui.Normal().Msg("Uploading source...")
	unixTime := time.Now()
	stageResp, uploadError := c.API.AppSourcePatch(
		appRef.Namespace,
		appRef.Name,
		file,
		cfg.ProcessCmd,
	)
	if uploadError != nil {
		return errors.Wrap(uploadError, "uploading source patch")
	}
	c.ui.Normal().Msgf(
		"Uploaded in %dms, waiting for build...",
		time.Since(unixTime).Milliseconds(),
	)

	unixTime = time.Now()
	stagingError := stagingWait(log, c.API, appRef.Namespace, stageResp.Stage.ID)
	if stagingError != nil {
		return errors.Wrap(stagingError, "staging failed")
	}
	c.ui.Normal().Msgf(
		"Build done in %dms, waiting for pod...",
		time.Since(unixTime).Milliseconds(),
	)

	unixTime = time.Now()
	waitRunningError := c.waitAppRunning(ctx, appRef)
	if waitRunningError != nil {
		return errors.Wrap(waitRunningError, "waiting for app to become ready")
	}
	c.ui.Normal().Msgf(
		"Pod ready in %dms. Watching for file changes...",
		time.Since(unixTime).Milliseconds(),
	)

	hashes, hashDirError := hashDir(path)
	if hashDirError != nil {
		return errors.Wrap(hashDirError, "computing initial file hashes")
	}
	return saveHashes(stateFile, hashes)
}

func (c *EpinioClient) watchSync(
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
		buildTime := time.Now()
		runBuildError := runBuildCmd(cfg.BuildCmd, path)
		if runBuildError != nil {
			return errors.Wrap(runBuildError, "local build failed")
		}
		c.ui.Normal().Msgf(
			"Built in %dms",
			time.Since(buildTime).Milliseconds(),
		)

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

	tmpFile, createTempError := os.CreateTemp("", "epinio-sync-*.tar")
	if createTempError != nil {
		return errors.Wrap(createTempError, "creating temp tar file")
	}
	tmpName := tmpFile.Name()
	closeTempError := tmpFile.Close()
	if closeTempError != nil {
		return errors.Wrap(closeTempError, "closing temp tar file")
	}
	defer func() {
		removeTempError := os.Remove(tmpName)
		if removeTempError != nil && !os.IsNotExist(removeTempError) {
			c.Log.Error(removeTempError, "removing temp sync tar")
		}
	}()

	createTarError := createSyncTar(tmpName, tarBase, tarPaths)
	if createTarError != nil {
		return errors.Wrap(createTarError, "creating sync tar")
	}

	syncTarFile, openSyncTarError := os.Open(tmpName)
	if openSyncTarError != nil {
		return errors.Wrap(openSyncTarError, "opening sync tar")
	}
	defer func() {
		closeSyncTarError := syncTarFile.Close()
		if closeSyncTarError != nil {
			c.Log.Error(closeSyncTarError, "closing sync tar")
		}
	}()

	syncTime := time.Now()
	_, syncError := c.API.AppSync(
		appRef.Namespace,
		appRef.Name,
		syncTarFile,
		mode,
		dest,
		binaryName,
	)
	if syncError != nil {
		syncErrStr := syncError.Error()
		noReadyPod := strings.Contains(syncErrStr, "no ready pod")
		serviceUnavailable := strings.Contains(syncErrStr, "503")
		if noReadyPod || serviceUnavailable {
			c.ui.Normal().Msg(
				"No ready pod -- clearing state for full push on next cycle...",
			)
			rmStateError := os.Remove(stateFile)
			if rmStateError != nil && !os.IsNotExist(rmStateError) {
				return errors.Wrap(rmStateError, "removing state file")
			}
			return nil
		}
		return errors.Wrap(syncError, "syncing to pod")
	}
	c.ui.Normal().Msgf(
		"Synced in %dms (via API)",
		time.Since(syncTime).Milliseconds(),
	)

	return saveHashes(stateFile, current)
}

// waitAppRunning polls the /running endpoint until the app is up or ctx is
// cancelled.
func (c *EpinioClient) waitAppRunning(
	ctx context.Context,
	appRef models.AppRef,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, runningError := c.API.AppRunning(appRef)
		if runningError == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// loadSyncConfig reads .epinio-sync.yaml from dir. Missing file is not an
// error.
func loadSyncConfig(dir string) (syncConfig, error) {
	var cfg syncConfig
	data, readFileError := os.ReadFile(filepath.Join(dir, watchConfigFile))
	if os.IsNotExist(readFileError) {
		return cfg, nil
	}
	if readFileError != nil {
		return cfg, readFileError
	}
	unmarshalError := yaml.Unmarshal(data, &cfg)
	return cfg, unmarshalError
}

// hashDir md5-hashes every non-excluded file under dir, returning a map of
// relative path to hex digest. Exclusions come from .gitignore and
// .epinioignore (the same files the push command respects), plus the
// watch-specific state files.
func hashDir(dir string) (fileHashes, error) {
	hashes := make(fileHashes)

	gitignorePatterns, readGitignoreError := readIgnoreFile(
		filepath.Join(dir, ".gitignore"),
	)
	if readGitignoreError != nil {
		return nil, fmt.Errorf("reading .gitignore: %w", readGitignoreError)
	}
	basePatterns := append( //nolint:gocritic
		gitignorePatterns,
		watchStateFile,
		watchConfigFile,
	)

	matcher, loadIgnoreError := helpers.LoadIgnoreMatcher(dir, basePatterns)
	if loadIgnoreError != nil {
		return nil, fmt.Errorf(
			"loading ignore patterns: %w",
			loadIgnoreError,
		)
	}

	walkError := filepath.WalkDir(dir, func(
		filePath string,
		dirEntry fs.DirEntry,
		entryError error,
	) error {
		if entryError != nil {
			return entryError
		}
		// Always skip .git, not a user-visible ignore, just implementation noise.
		if dirEntry.IsDir() && dirEntry.Name() == ".git" {
			return filepath.SkipDir
		}

		if matcher.ShouldIgnore(dir, filePath, dirEntry.IsDir()) {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if dirEntry.IsDir() {
			return nil
		}

		rel, getRelError := filepath.Rel(dir, filePath)
		if getRelError != nil {
			return getRelError
		}

		digest, hashFileError := md5File(filePath)
		if hashFileError != nil {
			return fmt.Errorf("hashing %s: %w", rel, hashFileError)
		}

		hashes[rel] = digest
		return nil
	})
	return hashes, walkError
}

// readIgnoreFile reads pattern lines from a gitignore-style file, stripping
// blank lines and comments. Missing file is silently ignored.
func readIgnoreFile(path string) ([]string, error) {
	data, readFileError := os.ReadFile(path)
	if os.IsNotExist(readFileError) {
		return nil, nil
	}
	if readFileError != nil {
		return nil, readFileError
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
	file, openFileError := os.Open(path)
	if openFileError != nil {
		return "", openFileError
	}
	defer func() {
		closeError := file.Close()
		if closeError != nil {
			log.Printf("md5File: failed to close %s: %v", path, closeError)
		}
	}()
	hasher := md5.New() // #nosec G401
	_, copyError := io.Copy(hasher, file)
	if copyError != nil {
		return "", copyError
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func loadHashes(path string) (fileHashes, error) {
	data, readFileError := os.ReadFile(path)
	if os.IsNotExist(readFileError) {
		return nil, nil
	}
	if readFileError != nil {
		return nil, readFileError
	}
	var hashes fileHashes
	unmarshalError := json.Unmarshal(data, &hashes)
	return hashes, unmarshalError
}

func saveHashes(path string, hashes fileHashes) error {
	data, marshalError := json.Marshal(hashes)
	if marshalError != nil {
		return marshalError
	}
	writeError := os.WriteFile(path, data, 0600)
	return writeError
}

// diffHashes returns the relative paths of changed/new files and deleted
// files.
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

// createSyncTar creates a plain (non-gzip) tar at destPath containing the
// listed files read from baseDir. Files are stored with their basenames to
// match what the server-side exec command expects.
func createSyncTar(destPath, baseDir string, files []string) error {
	outputFile, createFileError := os.Create(destPath)
	if createFileError != nil {
		return createFileError
	}
	defer func() {
		closeOutputError := outputFile.Close()
		if closeOutputError != nil {
			log.Printf(
				"createSyncTar: failed to close %s: %v",
				destPath,
				closeOutputError,
			)
		}
	}()

	tarWriter := tar.NewWriter(outputFile)
	defer func() {
		closeTarError := tarWriter.Close()
		if closeTarError != nil {
			log.Printf(
				"createSyncTar: failed to close tar writer: %v",
				closeTarError,
			)
		}
	}()

	for _, name := range files {
		srcPath := filepath.Join(baseDir, name)
		fileInfo, statFileError := os.Stat(srcPath)
		if statFileError != nil {
			return fmt.Errorf("stat %s: %w", name, statFileError)
		}

		header := &tar.Header{
			Name:    name,
			Mode:    int64(fileInfo.Mode()),
			Size:    fileInfo.Size(),
			ModTime: fileInfo.ModTime(),
		}
		writeHeaderError := tarWriter.WriteHeader(header)
		if writeHeaderError != nil {
			return fmt.Errorf(
				"writing tar header for %s: %w",
				name,
				writeHeaderError,
			)
		}

		srcFile, openSrcFileError := os.Open(srcPath)
		if openSrcFileError != nil {
			return fmt.Errorf("opening %s: %w", name, openSrcFileError)
		}

		_, copyError := io.Copy(tarWriter, srcFile)
		closeSrcFileError := srcFile.Close()
		if copyError != nil {
			return fmt.Errorf("writing %s to tar: %w", name, copyError)
		}
		if closeSrcFileError != nil {
			return fmt.Errorf("closing %s: %w", name, closeSrcFileError)
		}
	}
	return nil
}
