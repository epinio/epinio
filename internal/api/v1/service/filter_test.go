package service

import (
	"encoding/json"
	"testing"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func serviceFixture(name, catalogService string) models.Service {
	return models.Service{
		Meta:           models.Meta{Name: name},
		CatalogService: catalogService,
	}
}

func serviceNames(list models.ServiceList) []string {
	result := []string{}
	for _, service := range list {
		result = append(result, service.Meta.Name)
	}

	return result
}

func equal(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}

	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}

	return true
}

func TestFilterServices(t *testing.T) {
	list := models.ServiceList{
		serviceFixture("mysql-prod", "mysql-dev"),
		serviceFixture("MySQL-Staging", "mysql-dev"),
		serviceFixture("redis-prod", "redis-dev"),
		serviceFixture("mysql-ha-prod", "mysql-ha-dev"),
	}

	testCases := []struct {
		name           string
		search         string
		catalogService string
		expected       []string
	}{
		{
			name:     "no filters returns everything",
			expected: []string{"mysql-prod", "MySQL-Staging", "redis-prod", "mysql-ha-prod"},
		},
		{
			name:     "search is a case-insensitive substring of the name",
			search:   "MYSQL",
			expected: []string{"mysql-prod", "MySQL-Staging", "mysql-ha-prod"},
		},
		{
			name:           "catalog service is matched exactly, not as a prefix",
			catalogService: "mysql-dev",
			expected:       []string{"mysql-prod", "MySQL-Staging"},
		},
		{
			name:           "both filters are applied together",
			search:         "staging",
			catalogService: "mysql-dev",
			expected:       []string{"MySQL-Staging"},
		},
		{
			name:           "conflicting filters match nothing",
			search:         "redis",
			catalogService: "mysql-dev",
			expected:       []string{},
		},
		{
			name:           "unknown catalog service matches nothing",
			catalogService: "no-such-catalog",
			expected:       []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			filtered := filterServices(
				list,
				testCase.search,
				testCase.catalogService,
			)

			actual := serviceNames(filtered)
			if !equal(actual, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, actual)
			}
		})
	}
}

// A zero-match filter has to reach the client as [], not null: the dashboard
// parses a null list as a single resource.
func TestFilterServicesMarshalsEmptyResultAsArray(t *testing.T) {
	filtered := filterServices(
		models.ServiceList{serviceFixture("mysql-prod", "mysql-dev")},
		"",
		"no-such-catalog",
	)

	encoded, err := json.Marshal(filtered)
	if err != nil {
		t.Fatalf("marshalling filtered list: %s", err)
	}

	if string(encoded) != "[]" {
		t.Fatalf("expected [], got %s", encoded)
	}
}

func TestFilterServicesHandlesNilList(t *testing.T) {
	filtered := filterServices(nil, "", "")

	if filtered == nil {
		t.Fatal("expected an empty list, got nil")
	}
	if len(filtered) != 0 {
		t.Fatalf("expected an empty list, got %v", serviceNames(filtered))
	}
}
