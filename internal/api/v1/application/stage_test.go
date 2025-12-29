package application

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestJobDoneStateSuccess(t *testing.T) {
	job := batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	done, success := jobDoneState([]batchv1.Job{job})
	if !done || !success {
		t.Fatalf("expected done=true, success=true got done=%v success=%v", done, success)
	}
}

func TestJobDoneStateFailure(t *testing.T) {
	job := batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
			},
		},
	}

	done, success := jobDoneState([]batchv1.Job{job})
	if !done || success {
		t.Fatalf("expected done=true, success=false got done=%v success=%v", done, success)
	}
}

func TestJobDoneStatePending(t *testing.T) {
	job := batchv1.Job{}

	done, success := jobDoneState([]batchv1.Job{job})
	if done || success {
		t.Fatalf("expected done=false, success=false got done=%v success=%v", done, success)
	}
}
