{{/*
Expand the name of the chart.
*/}}
{{- define "ociregistry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ociregistry.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "ociregistry.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ociregistry.labels" -}}
helm.sh/chart: {{ include "ociregistry.chart" . }}
{{ include "ociregistry.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ociregistry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ociregistry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "ociregistry.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ociregistry.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image URL
*/}}
{{- define "ociregistry.image" -}}
{{- $sep := ":" }}
{{- $ref := .img.tag }}
{{- if ne (default "" .img.digest) "" }}
{{- $sep = "@" }}
{{- $ref = .img.digest }}
{{- end }}
{{- if ne .img.registry "" }}
{{- printf "%s/%s%s%s" .img.registry .img.repository $sep $ref }}
{{- else }}
{{- printf "%s%s%s" .img.repository $sep $ref }}
{{- end }}
{{- end }}
