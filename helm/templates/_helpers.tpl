{{/*
Expand the name of the chart.
*/}}
{{- define "wimp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
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

{{/* Control-plane resource name */}}
{{- define "wimp.cpFullname" -}}
{{- printf "%s-control-plane" (include "wimp.fullname" .) }}
{{- end }}

{{/* Web resource name */}}
{{- define "wimp.webFullname" -}}
{{- printf "%s-web" (include "wimp.fullname" .) }}
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
Base selector labels (no component — used in labels block only)
*/}}
{{- define "wimp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "wimp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/* Control-plane pod selector */}}
{{- define "wimp.cpSelectorLabels" -}}
{{ include "wimp.selectorLabels" . }}
app.kubernetes.io/component: control-plane
{{- end }}

{{/* Web pod selector */}}
{{- define "wimp.webSelectorLabels" -}}
{{ include "wimp.selectorLabels" . }}
app.kubernetes.io/component: web
{{- end }}

{{/*
Loki host — override via config.lokiHost, otherwise point to an external host.
*/}}
{{- define "wimp.lokiHost" -}}
{{- .Values.config.lokiHost }}
{{- end }}
