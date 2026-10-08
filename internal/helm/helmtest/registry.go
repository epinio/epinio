// Package helmtest provides test support for code working with helm charts in OCI registries.
package helmtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Registry is a minimal in-memory OCI registry, just enough for helm to push charts to it, and
// to pull and resolve them. It runs with TLS, using a self-signed certificate, like Epinio's own
// registry, and demands basic auth.
type Registry struct {
	server   *httptest.Server
	username string
	password string

	failPushes bool

	mu        sync.Mutex
	blobs     map[string][]byte
	uploads   map[string][]byte
	manifests map[string]manifest // keyed by `<repository>:<tag>` and `<repository>@<digest>`
}

type manifest struct {
	contentType string
	digest      string
	data        []byte
}

var (
	uploadRe   = regexp.MustCompile(`^/v2/(.+)/blobs/uploads/([^/]*)$`)
	blobRe     = regexp.MustCompile(`^/v2/(.+)/blobs/(sha256:[0-9a-f]+)$`)
	manifestRe = regexp.MustCompile(`^/v2/(.+)/manifests/([^/]+)$`)
	tagsRe     = regexp.MustCompile(`^/v2/(.+)/tags/list$`)
)

func NewRegistry(username, password string) *Registry {
	r := &Registry{
		username:  username,
		password:  password,
		blobs:     map[string][]byte{},
		uploads:   map[string][]byte{},
		manifests: map[string]manifest{},
	}
	r.server = httptest.NewTLSServer(http.HandlerFunc(r.serve))
	return r
}

// SetPassword changes the password the registry demands.
func (r *Registry) SetPassword(password string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.password = password
}

// FailPushes makes the registry reject the manifest of any pushed chart, or accept them again.
func (r *Registry) FailPushes(fail bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failPushes = fail
}

// Close shuts the registry down.
func (r *Registry) Close() {
	r.server.Close()
}

// Host returns the `host:port` of the registry. The loopback address is called by name, as
// Epinio's registry URL detection ignores `127.0.0.1`. The certificate of the registry cannot be
// verified, so code using it has to treat it as an in-cluster registry, which `localhost` is.
func (r *Registry) Host() string {
	_, port, err := net.SplitHostPort(r.server.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return "localhost:" + port
}

// URL returns the address of the registry with scheme, as it appears in registry credentials. The
// scheme is what makes code deleting images use https, instead of assuming http for localhost.
func (r *Registry) URL() string {
	return "https://" + r.Host()
}

// HasChart reports whether a manifest is stored for the tag of the repository.
func (r *Registry) HasChart(repository, tag string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, found := r.manifests[repository+":"+tag]
	return found
}

// Tags returns the sorted tags of the repository.
func (r *Registry) Tags(repository string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tags(repository)
}

func (r *Registry) tags(repository string) []string {
	tags := []string{}
	prefix := repository + ":"
	for key := range r.manifests {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			tags = append(tags, key[len(prefix):])
		}
	}
	sort.Strings(tags)
	return tags
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// writeJSON writes the value as the JSON body of the response. The failure to do so is only
// reported, as a test has nothing better to do about it.
func writeJSON(w http.ResponseWriter, value any) {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("helmtest registry: writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, map[string]any{"errors": []map[string]string{{"code": code, "message": code}}})
}

func (r *Registry) serve(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, pass, ok := req.BasicAuth()
	if !ok || user != r.username || pass != r.password {
		w.Header().Set("WWW-Authenticate", `Basic realm="fake registry"`)
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	path := req.URL.Path

	if path == "/v2/" || path == "/v2" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if m := uploadRe.FindStringSubmatch(path); m != nil {
		r.serveUpload(w, req, m[1], m[2])
		return
	}
	if m := blobRe.FindStringSubmatch(path); m != nil {
		r.serveBlob(w, m[2])
		return
	}
	if m := manifestRe.FindStringSubmatch(path); m != nil {
		r.serveManifest(w, req, m[1], m[2])
		return
	}
	if m := tagsRe.FindStringSubmatch(path); m != nil {
		tags := r.tags(m[1])
		if len(tags) == 0 {
			writeError(w, http.StatusNotFound, "NAME_UNKNOWN")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{"name": m[1], "tags": tags})
		return
	}

	writeError(w, http.StatusNotFound, "UNSUPPORTED")
}

func (r *Registry) serveUpload(w http.ResponseWriter, req *http.Request, repository, session string) {
	switch req.Method {
	case http.MethodPost:
		session = strconv.Itoa(len(r.uploads) + 1)
		r.uploads[session] = nil
		w.Header().Set("Location", "/v2/"+repository+"/blobs/uploads/"+session)
		w.WriteHeader(http.StatusAccepted)

	case http.MethodPatch, http.MethodPut:
		body, err := io.ReadAll(req.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BLOB_UPLOAD_INVALID")
			return
		}
		data := append(r.uploads[session], body...)
		r.uploads[session] = data

		if req.Method == http.MethodPatch {
			w.Header().Set("Location", "/v2/"+repository+"/blobs/uploads/"+session)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		digest := req.URL.Query().Get("digest")
		if digest != digestOf(data) {
			writeError(w, http.StatusBadRequest, "DIGEST_INVALID")
			return
		}
		r.blobs[digest] = data
		delete(r.uploads, session)
		w.Header().Set("Location", "/v2/"+repository+"/blobs/"+digest)
		w.Header().Set("Docker-Content-Digest", digest)
		w.WriteHeader(http.StatusCreated)

	default:
		writeError(w, http.StatusMethodNotAllowed, "UNSUPPORTED")
	}
}

func (r *Registry) serveBlob(w http.ResponseWriter, digest string) {
	data, found := r.blobs[digest]
	if !found {
		writeError(w, http.StatusNotFound, "BLOB_UNKNOWN")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Docker-Content-Digest", digest)
	_, _ = w.Write(data)
}

func (r *Registry) serveManifest(w http.ResponseWriter, req *http.Request, repository, reference string) {
	switch req.Method {
	case http.MethodPut:
		if r.failPushes {
			writeError(w, http.StatusInternalServerError, "UNKNOWN")
			return
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "MANIFEST_INVALID")
			return
		}
		manifest := manifest{
			contentType: req.Header.Get("Content-Type"),
			digest:      digestOf(body),
			data:        body,
		}
		r.manifests[repository+"@"+manifest.digest] = manifest
		if reference != manifest.digest {
			r.manifests[repository+":"+reference] = manifest
		}
		w.Header().Set("Docker-Content-Digest", manifest.digest)
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		// Manifests are deleted by digest. The tags pointing to it go with it.
		if _, found := r.manifests[repository+"@"+reference]; !found {
			writeError(w, http.StatusNotFound, "MANIFEST_UNKNOWN")
			return
		}
		for key, m := range r.manifests {
			if (m.digest == reference && strings.HasPrefix(key, repository+":")) || key == repository+"@"+reference {
				delete(r.manifests, key)
			}
		}
		w.WriteHeader(http.StatusAccepted)

	case http.MethodGet, http.MethodHead:
		key := repository + ":" + reference
		if strings.HasPrefix(reference, "sha256:") {
			key = repository + "@" + reference
		}
		manifest, found := r.manifests[key]
		if !found {
			writeError(w, http.StatusNotFound, "MANIFEST_UNKNOWN")
			return
		}
		w.Header().Set("Content-Type", manifest.contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(manifest.data)))
		w.Header().Set("Docker-Content-Digest", manifest.digest)
		_, _ = w.Write(manifest.data)

	default:
		writeError(w, http.StatusMethodNotAllowed, "UNSUPPORTED")
	}
}

// IsolateHelmConfig points helm at a fresh, empty config directory, and returns a function
// restoring the previous environment. Call it before the first helm client is created, as the
// clients are cached.
//
// It keeps tests from reading and writing the registry logins of the user running them. Setting
// only HELM_REGISTRY_CONFIG is not enough. The client used by `helm push` does not honor it, it
// reads the credentials from the default location below the config home. A login made through
// one client is then invisible to the push through the other.
func IsolateHelmConfig() (func(), error) {
	dir, err := os.MkdirTemp("", "epinio-helm-config-")
	if err != nil {
		return nil, err
	}

	vars := map[string]string{
		"HELM_CONFIG_HOME":     filepath.Join(dir, "config"),
		"HELM_CACHE_HOME":      filepath.Join(dir, "cache"),
		"HELM_DATA_HOME":       filepath.Join(dir, "data"),
		"HELM_REGISTRY_CONFIG": filepath.Join(dir, "config", "registry", "config.json"),
	}

	type previous struct {
		value string
		set   bool
	}
	old := map[string]previous{}

	restore := func() {
		for name, p := range old {
			if p.set {
				_ = os.Setenv(name, p.value)
			} else {
				_ = os.Unsetenv(name)
			}
		}
		_ = os.RemoveAll(dir)
	}

	for name, value := range vars {
		v, set := os.LookupEnv(name)
		old[name] = previous{v, set}
		if err := os.Setenv(name, value); err != nil {
			restore()
			return nil, err
		}
	}

	return restore, nil
}
