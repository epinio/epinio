package buildpack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func TestVerifyBuildpackNormalizationAndValidation(t *testing.T) {
	previous := repositoryExistsInRegistryFn
	t.Cleanup(func() { repositoryExistsInRegistryFn = previous })

	repositoryExistsInRegistryFn = func(_ context.Context, _ string, repository string) (bool, error) {
		return repository == "paketobuildpacks/nodejs", nil
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/buildpacks/verify?name=paketo-buildpacks/nodejs", nil)
	c.Request = req

	apiErr := Verify(c)
	if apiErr != nil {
		t.Fatalf("Verify returned unexpected api error: %+v", apiErr)
	}
	if w.Code != 200 {
		t.Fatalf("Verify status code = %d, want 200", w.Code)
	}

	var body models.BuildpackVerifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode verify response: %v", err)
	}
	if !body.Valid {
		t.Fatalf("expected valid response, got %+v", body)
	}
	if body.NormalizedName != "paketobuildpacks/nodejs" {
		t.Fatalf("normalized name = %q, want %q", body.NormalizedName, "paketobuildpacks/nodejs")
	}
}

func TestVerifyBuildpackReturnsServiceUnavailableOnRegistryError(t *testing.T) {
	previous := repositoryExistsInRegistryFn
	t.Cleanup(func() { repositoryExistsInRegistryFn = previous })

	repositoryExistsInRegistryFn = func(_ context.Context, _ string, _ string) (bool, error) {
		return false, errors.New("upstream unavailable")
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/buildpacks/verify?name=paketo-buildpacks/nodejs", nil)
	c.Request = req

	apiErr := Verify(c)
	if apiErr == nil {
		t.Fatalf("expected api error, got nil")
	}
	if apiErr.FirstStatus() != 503 {
		t.Fatalf("verify api status = %d, want 503", apiErr.FirstStatus())
	}
}

func TestSearchBuildpacksReturnsServiceUnavailableOnSearchError(t *testing.T) {
	previous := searchCNBRegistryFn
	t.Cleanup(func() { searchCNBRegistryFn = previous })

	searchCNBRegistryFn = func(context.Context, string) (*models.BuildpackSearchResponse, error) {
		return nil, errors.New("github timeout")
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/api/v1/buildpacks/search?q=node", nil)
	c.Request = req

	apiErr := Search(c)
	if apiErr == nil {
		t.Fatalf("expected api error, got nil")
	}
	if apiErr.FirstStatus() != 503 {
		t.Fatalf("search api status = %d, want 503", apiErr.FirstStatus())
	}
}
