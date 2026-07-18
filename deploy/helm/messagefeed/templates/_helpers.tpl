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

{{/* 为所有工作负载提供统一节点选择、软反亲和和拓扑分布约束。 */}}
{{- define "messagefeed.podScheduling" -}}
{{- $root := .root -}}
{{- $name := .name -}}
{{- $config := default (dict) .config -}}
{{- $nodeSelector := default $root.Values.scheduling.nodeSelector $config.nodeSelector -}}
{{- $tolerations := default $root.Values.scheduling.tolerations $config.tolerations -}}
{{- $affinity := default (dict) $config.affinity -}}
{{- with $nodeSelector }}
nodeSelector:
{{ toYaml . | indent 2 }}
{{- end }}
{{- with $tolerations }}
tolerations:
{{ toYaml . | indent 2 }}
{{- end }}
{{- if $root.Values.scheduling.topologySpread.enabled }}
topologySpreadConstraints:
  - maxSkew: {{ $root.Values.scheduling.topologySpread.maxSkew }}
    topologyKey: {{ $root.Values.scheduling.topologySpread.topologyKey | quote }}
    whenUnsatisfiable: {{ $root.Values.scheduling.topologySpread.whenUnsatisfiable }}
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: {{ $name }}
{{- end }}
{{- if $affinity }}
affinity:
{{ toYaml $affinity | indent 2 }}
{{- else if $root.Values.scheduling.podAntiAffinity.enabled }}
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: {{ $root.Values.scheduling.podAntiAffinity.weight }}
        podAffinityTerm:
          topologyKey: {{ $root.Values.scheduling.podAntiAffinity.topologyKey | default "kubernetes.io/hostname" | quote }}
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: {{ $name }}
{{- end }}
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
