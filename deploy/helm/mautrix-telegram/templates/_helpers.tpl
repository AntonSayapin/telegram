{{/*
Expand the chart name.
*/}}
{{- define "mautrix-telegram.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "mautrix-telegram.fullname" -}}
{{- default (include "mautrix-telegram.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "mautrix-telegram.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "mautrix-telegram.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: mautrix-telegram
{{- end -}}

{{/*
Selector labels. Keep app=mautrix-telegram for compatibility with the existing Service.
*/}}
{{- define "mautrix-telegram.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mautrix-telegram.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: mautrix-telegram
{{- end -}}
