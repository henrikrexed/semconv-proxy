{{/*
Expand the name of the chart. Defaults to the app name (semconv-proxy), not the chart name,
so resource names stay stable even though the chart artifact is published as
"semconv-proxy-chart" to avoid GHCR namespace collisions with the container image.
*/}}
{{- define "semconv-proxy-chart.name" -}}
{{- default "semconv-proxy" .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "semconv-proxy-chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := include "semconv-proxy-chart.name" . }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart label (helm.sh/chart). Uses the chart name + version.
*/}}
{{- define "semconv-proxy-chart.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "semconv-proxy-chart.labels" -}}
helm.sh/chart: {{ include "semconv-proxy-chart.chart" . }}
{{ include "semconv-proxy-chart.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: semconv-proxy
{{- end }}

{{/*
Selector labels. Stable across chart renames — keyed on the app name.
*/}}
{{- define "semconv-proxy-chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "semconv-proxy-chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "semconv-proxy-chart.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "semconv-proxy-chart.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference helper.
*/}}
{{- define "semconv-proxy-chart.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}
