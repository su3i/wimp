{{/*
Expand the name of the chart.
*/}}
{{- define "wimp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
If release name already contains the chart name, use it as-is.
*/}}
{{- define "wimp.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "wimp.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | trunc 63 | trimSuffix "-" }}
{{ include "wimp.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "wimp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "wimp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Loki host — uses the subchart service name when loki is enabled.
Override via config.lokiHost.
*/}}
{{- define "wimp.lokiHost" -}}
{{- if .Values.config.lokiHost }}
{{- .Values.config.lokiHost }}
{{- else }}
{{- printf "%s-loki" .Release.Name }}
{{- end }}
{{- end }}
