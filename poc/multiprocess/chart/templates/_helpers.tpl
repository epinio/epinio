{{- define "epinio-multiprocess.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "epinio-multiprocess.resourceName" -}}
{{- printf "%s-%s" .root.Values.epinio.appName .process | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "epinio-multiprocess.containerName" -}}
{{- .Values.epinio.appName | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "epinio-multiprocess.serviceName" -}}
{{- .Values.epinio.appName | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "epinio-multiprocess.labels" -}}
app.kubernetes.io/managed-by: epinio
app.kubernetes.io/part-of: {{ .root.Release.Namespace | quote }}
app.kubernetes.io/name: {{ .root.Values.epinio.appName | quote }}
app.kubernetes.io/component: application
epinio.io/process-name: {{ .process | quote }}
epinio.io/release-revision: {{ .root.Release.Revision | quote }}
helm.sh/chart: {{ include "epinio-multiprocess.chart" .root }}
{{- end -}}

{{- define "epinio-multiprocess.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Values.epinio.appName | quote }}
app.kubernetes.io/component: application
epinio.io/process-name: {{ .process | quote }}
{{- end -}}

{{- define "epinio-multiprocess.podMetadata" -}}
annotations:
  epinio.io/created-by: {{ .root.Values.epinio.username | quote }}
  {{- with .root.Values.epinio.start }}
  epinio.io/start: {{ . | quote }}
  {{- end }}
labels:
  {{- include "epinio-multiprocess.labels" . | nindent 2 }}
  epinio.io/stage-id: {{ .root.Values.epinio.stageID | quote }}
  epinio.io/app-container: {{ include "epinio-multiprocess.containerName" .root | quote }}
{{- end -}}

{{- define "epinio-multiprocess.podSpecPrefix" -}}
serviceAccountName: {{ .Release.Namespace }}
automountServiceAccountToken: true
{{- with .Values.epinio.configurations }}
volumes:
{{- range . }}
- name: {{ . }}
  secret:
    defaultMode: 420
    secretName: {{ . }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "epinio-multiprocess.container" -}}
name: {{ include "epinio-multiprocess.containerName" .root }}
image: {{ .root.Values.epinio.imageURL }}
imagePullPolicy: {{ .root.Values.image.pullPolicy }}
{{- if .root.Values.epinio.staged }}
command:
- "/cnb/lifecycle/launcher"
args:
- "--"
{{- range .process.command }}
- {{ . | quote }}
{{- end }}
{{- else }}
command:
  {{- toYaml .process.command | nindent 2 }}
{{- end }}
env:
- name: PORT
  value: "8080"
- name: EPINIO_PROCESS
  value: {{ .name | quote }}
{{- range .root.Values.epinio.env }}
- name: {{ .name | quote }}
  value: {{ .value | quote }}
{{- end }}
{{- with .root.Values.epinio.configpaths }}
volumeMounts:
{{- range . }}
- mountPath: /configurations/{{ .path }}
  name: {{ .name }}
  readOnly: true
{{- end }}
{{- end }}
{{- with .root.Values.resources }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}

{{- define "epinio-multiprocess.podScheduling" -}}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}
