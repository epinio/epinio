#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
cli=${EPINIO_CLI:-$repo_root/dist/epinio-linux-amd64}
namespace=${EPINIO_TEST_NAMESPACE:-poc}
app=${EPINIO_TEST_APP:-multiprocess-audit}
domain=${EPINIO_TEST_DOMAIN:-10.42.0.42.sslip.io}
route="$app.$namespace.$domain"
evidence_dir=${EPINIO_EVIDENCE_DIR:-$repo_root/poc/multiprocess/evidence/live}
selector="app.kubernetes.io/name=$app"
release=""

mkdir -p "$evidence_dir"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_eq() {
  local expected=$1
  local actual=$2
  local description=$3
  if [[ "$actual" != "$expected" ]]; then
    fail "$description: expected [$expected], got [$actual]"
  fi
}

assert_contains() {
  local expected=$1
  local actual=$2
  local description=$3
  if [[ "$actual" != *"$expected"* ]]; then
    fail "$description: expected [$actual] to contain [$expected]"
  fi
}

current_revision() {
  helm status "$release" -n "$namespace" -o json | jq -r '.version'
}

revision_status() {
  local revision=$1
  helm history "$release" -n "$namespace" -o json |
    jq -r --argjson revision "$revision" '.[] | select(.revision == $revision) | .status'
}

snapshot() {
  local case_name=$1
  local case_dir="$evidence_dir/$case_name"
  mkdir -p "$case_dir"

  kubectl -n "$namespace" get app "$app" -o yaml >"$case_dir/app.yaml"
  kubectl -n "$namespace" get deployment,replicaset,service,ingress,cronjob,job,pod,certificate \
    -l "$selector" -o yaml >"$case_dir/resources.yaml"
  kubectl -n "$namespace" get deployment,replicaset,service,ingress,cronjob,job,pod,certificate \
    -l "$selector" -o wide >"$case_dir/resources.txt"
  helm history "$release" -n "$namespace" -o json >"$case_dir/helm-history.json"
  helm get values "$release" -n "$namespace" -o yaml >"$case_dir/helm-values.yaml"
  helm get manifest "$release" -n "$namespace" >"$case_dir/helm-manifest.yaml"
  helm get hooks "$release" -n "$namespace" >"$case_dir/helm-hooks.yaml"
  kubectl -n "$namespace" get events --sort-by=.metadata.creationTimestamp | \
    rg "$app" >"$case_dir/events.txt" || true
}

run_push() {
  local case_name=$1
  local manifest=$2
  local expectation=$3
  local case_dir="$evidence_dir/$case_name"
  mkdir -p "$case_dir"

  echo "[$case_name] pushing $manifest (expect $expectation)"
  set +e
  /usr/bin/time -f 'elapsed=%e exit=%x' \
    "$cli" push --name "$app" --route "$route" "$repo_root/poc/multiprocess/$manifest" \
    >"$case_dir/push.log" 2>&1
  push_rc=$?
  set -e

  echo "[$case_name] push exit=$push_rc"
  if [[ "$expectation" == success && "$push_rc" -ne 0 ]]; then
    tail -80 "$case_dir/push.log" >&2
    fail "$case_name push unexpectedly failed"
  fi
  if [[ "$expectation" == failure && "$push_rc" -eq 0 ]]; then
    fail "$case_name push unexpectedly succeeded"
  fi
}

assert_deployment() {
  local process=$1
  local replicas=$2
  local image=$3
  local expected_command_json=$4
  local name="$app-$process"

  kubectl -n "$namespace" rollout status "deployment/$name" --timeout=2m >/dev/null
  assert_eq "$replicas" "$(kubectl -n "$namespace" get deployment "$name" -o jsonpath='{.spec.replicas}')" "$name replicas"
  assert_eq "$replicas" "$(kubectl -n "$namespace" get deployment "$name" -o jsonpath='{.status.readyReplicas}')" "$name ready replicas"
  assert_eq "$image" "$(kubectl -n "$namespace" get deployment "$name" -o jsonpath='{.spec.template.spec.containers[0].image}')" "$name image"
  assert_eq "$expected_command_json" "$(kubectl -n "$namespace" get deployment "$name" -o json | jq -c '.spec.template.spec.containers[0].command')" "$name command"
}

assert_cronjob() {
  local image=$1
  local expected_command_json=$2
  local name="$app-scheduler"

  assert_eq '* * * * *' "$(kubectl -n "$namespace" get cronjob "$name" -o jsonpath='{.spec.schedule}')" "$name schedule"
  assert_eq "$image" "$(kubectl -n "$namespace" get cronjob "$name" -o jsonpath='{.spec.jobTemplate.spec.template.spec.containers[0].image}')" "$name image"
  assert_eq "$expected_command_json" "$(kubectl -n "$namespace" get cronjob "$name" -o json | jq -c '.spec.jobTemplate.spec.template.spec.containers[0].command')" "$name command"
}

assert_release_job() {
  local revision=$1
  local expected_condition=$2
  local image=$3
  local log_pattern=$4
  local case_dir=$5
  local name="$app-release-r$revision"

  kubectl -n "$namespace" get job "$name" -o yaml >"$case_dir/$name.yaml"
  assert_eq "$image" "$(kubectl -n "$namespace" get job "$name" -o jsonpath='{.spec.template.spec.containers[0].image}')" "$name image"

  if [[ "$expected_condition" == complete ]]; then
    kubectl -n "$namespace" wait --for=condition=complete "job/$name" --timeout=30s >/dev/null
  else
    kubectl -n "$namespace" wait --for=condition=failed "job/$name" --timeout=30s >/dev/null
  fi

  kubectl -n "$namespace" logs "job/$name" >"$case_dir/$name.log"
  assert_contains "$log_pattern" "$(cat "$case_dir/$name.log")" "$name logs"

  kubectl -n "$namespace" get pod \
    -l "$selector,epinio.io/process-name=release,epinio.io/release-revision=$revision" \
    -o json >"$case_dir/$name-pods.json"

  if [[ "$expected_condition" == failed ]]; then
    assert_eq 42 "$(jq -r '[.items[].status.containerStatuses[]?.state.terminated.exitCode] | unique | .[0]' \
      "$case_dir/$name-pods.json")" "$name container exit code"
  fi
}

assert_no_release_job() {
  local revision=$1
  local case_dir=$2
  local name="$app-release-r$revision"

  if kubectl -n "$namespace" get job "$name" >"$case_dir/$name-lookup.txt" 2>&1; then
    fail "$name exists, but Helm's automatic atomic rollback was expected to skip hooks"
  fi
  echo "not created: automatic atomic rollback skipped release hooks" >"$case_dir/$name-lookup.txt"
}

assert_scheduled_cron_run() {
  local revision=$1
  local image=$2
  local log_pattern=$3
  local case_dir=$4
  local deadline=$((SECONDS + 100))
  local job=""

  echo "[cron] waiting for controller-created revision $revision job"
  while (( SECONDS < deadline )); do
    job=$(kubectl -n "$namespace" get job \
      -l "$selector,epinio.io/process-name=scheduler,epinio.io/release-revision=$revision" \
      -o json | jq -r --arg image "$image" '
        [.items[]
          | select(.metadata.ownerReferences[]?.kind == "CronJob")
          | select(.spec.template.spec.containers[0].image == $image)]
        | sort_by(.metadata.creationTimestamp)
        | last
        | .metadata.name // empty')
    if [[ -n "$job" ]]; then
      break
    fi
    sleep 2
  done

  [[ -n "$job" ]] || fail "no controller-created cron Job for revision $revision and image $image"
  kubectl -n "$namespace" wait --for=condition=complete "job/$job" --timeout=30s >/dev/null
  kubectl -n "$namespace" get job "$job" -o yaml >"$case_dir/$job.yaml"
  kubectl -n "$namespace" logs "job/$job" >"$case_dir/$job.log"
  assert_contains "$log_pattern" "$(cat "$case_dir/$job.log")" "$job logs"
  echo "[cron] observed $job: $(cat "$case_dir/$job.log")"
}

assert_route_and_service() {
  local expected_version=$1
  local case_dir=$2
  local ingress_tls_secret
  local certificate_name
  local response=""
  local port=18082

  assert_eq web "$(kubectl -n "$namespace" get service "$app" -o jsonpath='{.spec.selector.epinio\.io/process-name}')" "service route process"
  assert_eq "$route" "$(kubectl -n "$namespace" get ingress -l "$selector" -o jsonpath='{.items[0].spec.rules[0].host}')" "ingress host"
  ingress_tls_secret=$(kubectl -n "$namespace" get ingress -l "$selector" -o jsonpath='{.items[0].spec.tls[0].secretName}')
  [[ -n "$ingress_tls_secret" ]] || fail "Ingress TLS secret is empty"
  certificate_name=$(kubectl -n "$namespace" get certificate -l "$selector" -o jsonpath='{.items[0].metadata.name}')
  [[ -n "$certificate_name" ]] || fail "Certificate was not rendered"
  kubectl -n "$namespace" wait --for=condition=ready "certificate/$certificate_name" --timeout=90s >/dev/null
  kubectl -n "$namespace" get secret "$ingress_tls_secret" >/dev/null

  kubectl -n "$namespace" port-forward "service/$app" "$port:8080" >"$case_dir/port-forward.log" 2>&1 &
  local forward_pid=$!
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if response=$(curl --silent --show-error --max-time 3 "http://127.0.0.1:$port"); then
      break
    fi
    sleep 1
  done
  kill "$forward_pid" 2>/dev/null || true
  wait "$forward_pid" 2>/dev/null || true

  printf '%s\n' "$response" >"$case_dir/web-response.txt"
  assert_contains "web version=$expected_version process=web" "$response" "web response"
}

assert_prebuilt_state() {
  local version=$1
  local worker_replicas=$2
  local case_dir=$3
  local image="docker.io/epinio-poc/multiprocess:$version"

  assert_deployment web 2 "$image" '["python","/app/app.py","web"]'
  assert_deployment worker "$worker_replicas" "$image" '["python","/app/app.py","worker"]'
  assert_cronjob "$image" '["python","/app/app.py","cron"]'
  assert_route_and_service "$version" "$case_dir"
}

assert_candidate_fully_scaled_down() {
  local image=$1
  local deadline=$((SECONDS + 120))
  local pod_count
  local nonzero_rs

  echo "[rollback] waiting for candidate pods and ReplicaSets to converge to zero"
  while (( SECONDS < deadline )); do
    pod_count=$(kubectl -n "$namespace" get pod -l "$selector" -o json |
      jq -r --arg image "$image" '[.items[]
        | select(.metadata.ownerReferences[]?.kind == "ReplicaSet")
        | select(.spec.containers[0].image == $image)] | length')
    nonzero_rs=$(kubectl -n "$namespace" get replicaset -l "$selector" -o json |
      jq -r --arg image "$image" '[.items[]
        | select(.spec.template.spec.containers[0].image == $image)
        | select((.spec.replicas // 0) != 0 or (.status.replicas // 0) != 0)] | length')
    if [[ "$pod_count" == 0 && "$nonzero_rs" == 0 ]]; then
      echo "[rollback] candidate image fully scaled down"
      return 0
    fi
    sleep 2
  done

  fail "candidate image $image still has pods or nonzero ReplicaSets after rollback"
}

cd "$repo_root"

if [[ ! -x "$cli" ]]; then
  fail "Epinio CLI not executable: $cli"
fi
if kubectl -n "$namespace" get app "$app" >/dev/null 2>&1; then
  fail "test application $namespace/$app already exists; refusing to overwrite evidence"
fi

{
  date --utc --iso-8601=seconds
  git rev-parse HEAD
  kubectl version -o json | jq -c '{clientVersion:.clientVersion.gitVersion,serverVersion:.serverVersion.gitVersion}'
  helm version --short
  "$cli" info
} >"$evidence_dir/environment.txt" 2>&1

run_push 01-initial-v1 epinio-v1.yml success
release=$(kubectl -n "$namespace" get deployment "$app-web" -o jsonpath='{.metadata.annotations.meta\.helm\.sh/release-name}')
[[ -n "$release" ]] || fail "could not discover Helm release"
revision=$(current_revision)
assert_eq 1 "$revision" "initial Helm revision"
assert_eq deployed "$(revision_status "$revision")" "initial Helm status"
assert_prebuilt_state v1 2 "$evidence_dir/01-initial-v1"
assert_release_job "$revision" complete docker.io/epinio-poc/multiprocess:v1 'migration version=v1 fail=False' "$evidence_dir/01-initial-v1"
assert_scheduled_cron_run "$revision" docker.io/epinio-poc/multiprocess:v1 'cron ran version=v1' "$evidence_dir/01-initial-v1"
snapshot 01-initial-v1

run_push 02-upgrade-v2 epinio-v2.yml success
revision=$(current_revision)
assert_eq 2 "$revision" "upgrade Helm revision"
assert_eq deployed "$(revision_status "$revision")" "upgrade Helm status"
assert_prebuilt_state v2 3 "$evidence_dir/02-upgrade-v2"
assert_release_job "$revision" complete docker.io/epinio-poc/multiprocess:v2 'migration version=v2 fail=False' "$evidence_dir/02-upgrade-v2"
assert_scheduled_cron_run "$revision" docker.io/epinio-poc/multiprocess:v2 'cron ran version=v2' "$evidence_dir/02-upgrade-v2"
snapshot 02-upgrade-v2

run_push 03-failed-release epinio-migration-fail.yml failure
assert_eq 4 "$(current_revision)" "failed-release rollback revision"
assert_eq failed "$(revision_status 3)" "failed-release candidate status"
assert_eq deployed "$(revision_status 4)" "failed-release rollback status"
helm get values "$release" -n "$namespace" --revision 3 -o yaml >"$evidence_dir/03-failed-release/failed-values.yaml"
helm get hooks "$release" -n "$namespace" --revision 3 >"$evidence_dir/03-failed-release/failed-hooks.yaml"
assert_release_job 3 failed docker.io/epinio-poc/multiprocess:v3 'migration version=v3 fail=True' "$evidence_dir/03-failed-release"
assert_no_release_job 4 "$evidence_dir/03-failed-release"
assert_prebuilt_state v2 3 "$evidence_dir/03-failed-release"
assert_eq docker.io/epinio-poc/multiprocess:v3 "$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.imageurl}')" "failed-release desired App image"
assert_eq docker.io/epinio-poc/multiprocess:v2 "$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.origin.container}')" "failed-release persisted origin"
snapshot 03-failed-release

run_push 04-unhealthy-worker epinio-unhealthy.yml failure
assert_eq 6 "$(current_revision)" "unhealthy rollback revision"
assert_eq failed "$(revision_status 5)" "unhealthy candidate status"
assert_eq deployed "$(revision_status 6)" "unhealthy rollback status"
helm get values "$release" -n "$namespace" --revision 5 -o yaml >"$evidence_dir/04-unhealthy-worker/failed-values.yaml"
helm get manifest "$release" -n "$namespace" --revision 5 >"$evidence_dir/04-unhealthy-worker/failed-manifest.yaml"
assert_release_job 5 complete docker.io/epinio-poc/multiprocess:v4 'migration version=v4 fail=False' "$evidence_dir/04-unhealthy-worker"
assert_no_release_job 6 "$evidence_dir/04-unhealthy-worker"
assert_prebuilt_state v2 3 "$evidence_dir/04-unhealthy-worker"
candidate_rs=$(kubectl -n "$namespace" get replicaset -l "$selector,epinio.io/process-name=worker" -o json |
  jq -r '[.items[] | select(.spec.template.spec.containers[0].image == "docker.io/epinio-poc/multiprocess:v4")] | length')
[[ "$candidate_rs" -gt 0 ]] || fail "no v4 worker ReplicaSet remained as evidence of candidate rollout"
assert_candidate_fully_scaled_down docker.io/epinio-poc/multiprocess:v4
assert_eq docker.io/epinio-poc/multiprocess:v4 "$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.imageurl}')" "unhealthy desired App image"
assert_eq docker.io/epinio-poc/multiprocess:v2 "$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.origin.container}')" "unhealthy persisted origin"
snapshot 04-unhealthy-worker

run_push 05-staged-direct epinio-staged.yml success
revision=$(current_revision)
assert_eq 7 "$revision" "staged Helm revision"
assert_eq deployed "$(revision_status "$revision")" "staged Helm status"
stage_id=$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.stageid}')
app_image=$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.imageurl}')
[[ -n "$stage_id" ]] || fail "staged App stage ID is empty"
[[ "$app_image" == *":$stage_id" ]] || fail "staged App image does not use stage ID as its complete tag"
[[ -n "$(kubectl -n "$namespace" get app "$app" -o jsonpath='{.spec.origin.path}')" ]] || fail "successful staged push did not persist path origin"
runtime_image=$(kubectl -n "$namespace" get deployment "$app-web" -o jsonpath='{.spec.template.spec.containers[0].image}')
assert_deployment web 2 "$runtime_image" '["/cnb/lifecycle/launcher"]'
assert_deployment worker 3 "$runtime_image" '["/cnb/lifecycle/launcher"]'
assert_cronjob "$runtime_image" '["/cnb/lifecycle/launcher"]'
assert_eq '["--","python","app.py","web"]' "$(kubectl -n "$namespace" get deployment "$app-web" -o json | jq -c '.spec.template.spec.containers[0].args')" "staged web args"
assert_eq '["--","python","app.py","worker"]' "$(kubectl -n "$namespace" get deployment "$app-worker" -o json | jq -c '.spec.template.spec.containers[0].args')" "staged worker args"
assert_eq "$runtime_image" "$(kubectl -n "$namespace" get cronjob "$app-scheduler" -o jsonpath='{.spec.jobTemplate.spec.template.spec.containers[0].image}')" "staged cron image"
assert_release_job "$revision" complete "$runtime_image" 'migration version=staged-v5 fail=False' "$evidence_dir/05-staged-direct"
assert_scheduled_cron_run "$revision" "$runtime_image" 'cron ran version=staged-v5' "$evidence_dir/05-staged-direct"
assert_route_and_service staged-v5 "$evidence_dir/05-staged-direct"
snapshot 05-staged-direct

echo "$release" >"$evidence_dir/helm-release.txt"
echo "PASS: live initial, upgrade, cron, release failure, unhealthy rollback, and direct staged push matrix"
