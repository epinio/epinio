package gitea

// CreateRepoScript is the script for creating a gitea repo
const CreateRepoScript = `#!/bin/bash -e
target="{{ .Target }}"
user="{{ .Username }}"
pass="{{ .Password }}"
app_name="{{ .AppName }}"

curl -sSL -X POST "http://$user:$pass@gitea.$target/api/v1/user/repos" \
	-H  "accept: application/json" \
	-H "Content-Type: application/json" \
	-d "
	{
		\"auto_init\": true,
		\"default_branch\": \"main\",
		\"description\": \"automatically deployed app\",
		\"name\": \"$app_name\",
		\"private\": true,
		\"trust_model\": \"default\"
	}"
`

// EnableDroneScript enables drone for a gitea repo
const EnableDroneScript = `#!/bin/bash -e
target="{{ .Target }}"
user="{{ .Username }}"
pass="{{ .Password }}"
app_name="{{ .AppName }}"
drone_token="{{ .DroneToken }}"

curl -sSL "http://drone.$target/api/user/repos" \
	-X "POST" \
	-H "Connection: keep-alive" \
	-H "Content-Length: 0" \
	-H "Accept: */*" \
	-H "Authorization: Bearer $drone_token" > /dev/null

curl -sSL -X POST "http://drone.$target/api/repos/$user/$app_name" \
	-H "Authorization: Bearer $drone_token" \
	-H  "accept: application/json" \
	-H "Content-Type: application/json" > /dev/null
`

// DeleteAppScript deletes all traces of an app
const DeleteAppScript = `#!/bin/bash
  app_name="{{ .AppName }}"

  kubectl delete image -n eirini-workloads "$app_name"
  kubectl delete lrp -n eirini-workloads "$app_name"
  kubectl delete service -n eirini-workloads "$app_name"
  kubectl delete ingress -n eirini-workloads "$app_name"
`

// PrepareCodeScript generates support files for pushing an app
const PrepareCodeScript = `#!/bin/bash -e
  target="{{ .Target }}"
  user="{{ .Username }}"
  pass="{{ .Password }}"
  app_name="{{ .AppName }}"
  app_dir="{{ .AppDir }}"
  image_user="{{ .ImageUser }}"
  image_password="{{ .ImagePassword }}"

  cat <<EOF >> "$app_dir/.drone.yml"
kind: pipeline
type: kubernetes
name: $app_name

steps:
- name: kpack
  image: bitnami/kubectl
  commands:
  - kubectl apply -f .kube/*
EOF

  mkdir "$app_dir/.kube"

  cat <<EOF >> "$app_dir/.kube/app.yml"
---
apiVersion: kpack.io/v1alpha1
kind: Image
metadata:
  name: $app_name
  namespace: eirini-workloads
spec:
  tag: $image_user/carrier-$app_name
  serviceAccount: app-serviceaccount
  builder:
    name: carrier-builder
    kind: ClusterBuilder
  source:
    git:
      url: http://gitea.$target/$user/$app_name
      revision: main
---
# DEPLOYMENT
apiVersion: eirini.cloudfoundry.org/v1
kind: LRP
metadata:
  name: $app_name
  namespace: eirini-workloads
spec:
  GUID: "$app_name"
  version: "version-1"
  appName: "$app_name"
  instances: 1
  lastUpdated: "never"
  diskMB: 100
  runsAsRoot: true
  env:
    PORT: "8080"
  ports:
  - 8080
  image: "$image_user/carrier-$app_name"
  appRoutes:
  - hostname: $app_name.$target
    port: 8080
EOF
`

// PushScript is the script we execute when pushing an app to gitea
const PushScript = `#!/bin/bash -e
target="{{ .Target }}"
user="{{ .Username }}"
pass="{{ .Password }}"
app_name="{{ .AppName }}"
tmp_dir="{{ .AppDir }}"

cd "$tmp_dir"
touch $(date -u +'%Y%m%d%H%M%S')
git init
git remote add carrier "http://$user:$pass@gitea.$target/$user/$app_name"
git fetch --all
git reset --soft carrier/main
git add --all
git commit -m "pushed at $(date)"
git push carrier master:main
`

// DroneTokenScript is used to grab a drone token
const DroneTokenScript = `#!/bin/bash -e
target="{{ .Target }}"
gitea_user="{{ .Username }}"
gitea_password="{{ .Password }}"

# Login to gitea
cookie_jar=$(mktemp --tmpdir drone-cookies.XXX)
state=$(curl -sSLi "http://drone.$target/login" \
	--cookie-jar "$cookie_jar" | grep "_oauth_state_" | sed -e "s/[=;]/ /g" | awk '{print $3}')
csrf_token=$(curl -sSL "http://gitea.$target/user/login" \
	-H 'Content-Type: application/x-www-form-urlencoded' \
	--data-raw "user_name=${gitea_user}&password=${gitea_password}" \
	--cookie-jar "$cookie_jar" \
	--cookie "$cookie_jar" | grep csrf | grep -v content | awk '{print $2}' | sed -e "s/[',]//g")

csrf_token=$(cat "$cookie_jar" | grep _csrf | awk '{print $7}')

# Get token
curl -sSL "http://drone.$target/api/user/token" \
	-X 'POST' \
	-H 'Accept: */*' \
	--cookie "$cookie_jar" | jq -r ".token"
`

// StagingStatusScript returns a status for the app
const StagingStatusScript = `
app_name="{{ .AppName }}"
image_status=$(kubectl get image -n eirini-workloads $app_name -o json | jq -r '.status.conditions[] | select(.type == "Ready").status')

if [ "$image_status" = "True" ]; then
  echo "STAGED"
else
  echo "STAGING"
fi
`
