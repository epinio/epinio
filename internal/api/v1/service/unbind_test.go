package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/epinio/epinio/helpers/kubernetes"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteServiceBindingsAggregatesConfigNames(t *testing.T) {
	ctx := context.Background()

	var calls int
	var received []string

	stubDelete := func(_ context.Context, _ *kubernetes.Cluster, namespace, appName, userName string, configs []string) apierror.APIErrors {
		calls++

		if namespace != "test-ns" {
			t.Fatalf("expected namespace test-ns, got %s", namespace)
		}
		if appName != "my-app" {
			t.Fatalf("expected app my-app, got %s", appName)
		}
		if userName != "alice" {
			t.Fatalf("expected user alice, got %s", userName)
		}

		received = append([]string{}, configs...)
		return nil
	}

	serviceConfigurations := []v1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "cfg-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cfg-b"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cfg-c"}},
	}

	errs := deleteServiceBindings(ctx, nil, "test-ns", "my-app", "alice", serviceConfigurations, stubDelete)
	if errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if calls != 1 {
		t.Fatalf("expected deleteBinding to be called once, got %d", calls)
	}

	expected := []string{"cfg-a", "cfg-b", "cfg-c"}
	if !reflect.DeepEqual(received, expected) {
		t.Fatalf("expected %v, got %v", expected, received)
	}
}

func TestDeleteServiceBindingsSkipsEmptyConfigs(t *testing.T) {
	ctx := context.Background()

	var calls int
	stubDelete := func(_ context.Context, _ *kubernetes.Cluster, _, _, _ string, _ []string) apierror.APIErrors {
		calls++
		return nil
	}

	errs := deleteServiceBindings(ctx, nil, "test-ns", "my-app", "alice", nil, stubDelete)
	if errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if calls != 0 {
		t.Fatalf("expected deleteBinding not to run, got %d calls", calls)
	}
}
