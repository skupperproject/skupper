{{/*
Expand the name of the chart.
*/}}
{{- define "network-observer.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "network-observer.fullname" -}}
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
{{- define "network-observer.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "network-observer.labels" -}}
{{ include "network-observer.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- if not .Values.skipManagementLabels }}
helm.sh/chart: {{ include "network-observer.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
app.kubernetes.io/part-of: skupper-network-observer
{{- end }}

{{/*
Selector labels
*/}}
{{- define "network-observer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-observer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "network-observer.serviceAccountName" -}}
{{- if eq .Values.auth.strategy "openshift" -}}
{{- .Values.auth.openshift.serviceAccount.nameOverride | default (include "network-observer.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create the Skupper Certificate Name
*/}}
{{- define "network-observer.clientCertificateName" -}}
{{- .Values.router.certificate.nameOverride | default (printf "%s-client" (include "network-observer.fullname" .)) }}
{{- end }}

{{/*
Create the TLS Secret Name
*/}}
{{- define "network-observer.tlsSecretName" -}}
{{- .Values.tls.secretName | default (printf "%s-tls" (include "network-observer.fullname" .)) }}
{{- end }}

{{/*
Create the nginx configmap name
*/}}
{{- define "network-observer.nginxConfigMapName" -}}
{{- (printf "%s-nginx" (include "network-observer.fullname" .)) }}
{{- end }}

{{- define "network-observer.basicAuthSecretName" -}}
{{- .Values.auth.basic.secretName | default (printf "%s-auth" (include "network-observer.fullname" .)) }}
{{- end }}

{{- define "network-observer.sessionCookieSecretName" -}}
{{- .Values.auth.openshift.secretName | default (printf "%s-session" (include "network-observer.fullname" .)) }}
{{- end }}

{{- define "network-observer.setupJobName" -}}
{{- printf "%s-setup" (include "network-observer.fullname" .) }}
{{- end }}

{{- define "network-observer.clusterRoleName" -}}
{{- .Values.auth.openshift.bearerTokenAuth.clusterRoleName | default (include "network-observer.fullname" .) }}
{{- end }}

{{- define "network-observer.bearerTokenSecret" -}}
{{- printf "%s-servicemonitor" (include "network-observer.fullname" .) | trunc 63 | trimSuffix "-"}}
{{- end }}

{{- define "network-observer.serviceMonitorName" -}}
{{- .Values.serviceMonitor.nameOverride | default (include "network-observer.fullname" .) }}
{{- end }}

{{- define "network-observer.bearerTokenAuth" -}}
{{- with .Values.auth.openshift.bearerTokenAuth -}}
{{- if .enabled -}}
"/":
{{ if .resourceAttributes.namespaced }}  namespace: {{ $.Release.Namespace }}{{- end }}
{{ if .resourceAttributes.group }}  group: {{ .resourceAttributes.group }}{{- end }}
{{ if .resourceAttributes.resource }}  resource: {{ .resourceAttributes.resource }}{{- end }}
{{ if .resourceAttributes.verb }}  verb: {{ .resourceAttributes.verb }}{{- end }}
{{- end }}
{{- end }}
{{- end }}
