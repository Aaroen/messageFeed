{{/* 提供 Chart 资源命名、标签和 Secret 名称等共享模板。 */}}
{{- define "messagefeed.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "messagefeed.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else if contains (include "messagefeed.name" .) .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "messagefeed.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "messagefeed.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
app.kubernetes.io/part-of: messagefeed
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "messagefeed.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
{{- end }}

{{- define "messagefeed.postgresSecretName" -}}
{{- default (printf "%s-postgresql" (include "messagefeed.fullname" .)) .Values.postgresql.auth.existingSecret }}
{{- end }}

{{- define "messagefeed.appSecretName" -}}
{{- default (printf "%s-app" (include "messagefeed.fullname" .)) .Values.appSecret.existingSecret }}
{{- end }}

{{- define "messagefeed.cloudflaredSecretName" -}}
{{- default (printf "%s-cloudflared" (include "messagefeed.fullname" .)) .Values.cloudflared.existingSecret }}
{{- end }}

{{- define "messagefeed.caddySecretName" -}}
{{- default (printf "%s-caddy-certs" (include "messagefeed.fullname" .)) .Values.gateway.tls.existingSecret }}
{{- end }}

{{- define "messagefeed.databaseURL" -}}
{{- $user := .Values.postgresql.auth.username -}}
{{- $password := required "postgresql.auth.password is required when appSecret.existingSecret is empty" .Values.postgresql.auth.password | urlquery -}}
{{- $database := .Values.postgresql.auth.database -}}
{{- printf "postgres://%s:%s@messagefeed-postgres:5432/%s?sslmode=disable" $user $password $database -}}
{{- end }}
