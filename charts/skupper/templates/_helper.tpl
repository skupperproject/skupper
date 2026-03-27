{{/*
Controller Install Scope
Returns "true" if cluster-scoped, "false" if namespace-scoped.
*/}}
{{- define "skupper.clusterScoped" -}}
{{- if eq .Values.scope "cluster" -}}
true
{{- else if eq .Values.scope "namespace" -}}
false
{{- else if .Values.scope -}}
{{- fail "value for .Values.scope must be cluster or namespace" -}}
{{- else -}}
{{ printf "%t" .Values.rbac.clusterScoped -}}
{{- end -}}
{{- end }}

{{/*
Expand the name of the chart.
*/}}
{{- define "skupper.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "skupper.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "skupper.fullname" -}}
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
{{- define "skupper.labels" -}}
{{ include "skupper.selectorLabels" . }}
helm.sh/chart: {{ include "skupper.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/part-of: skupper
app.kubernetes.io/name: skupper-controller
{{- end }}

{{/*
Selector labels
*/}}
{{- define "skupper.selectorLabels" -}}
application: skupper-controller
{{- end }}

{{/*
ServiceAccount Name
*/}}
{{- define "skupper.serviceAccountName" -}}
{{- default (include "skupper.fullname" .) .Values.serviceAccount.name }}
{{- end }}

{{/*
Controller Image
*/}}
{{- define "skupper.controllerImage" -}}
{{- $deprecatedImageRef := .Values.controllerImage -}}
{{- $repositoryName := .Values.controller.repository -}}
{{- $tag := default .Chart.AppVersion .Values.controller.tag -}}
{{- $digest := .Values.controller.digest -}}
{{- if $deprecatedImageRef -}}
{{- $deprecatedImageRef  -}}
{{- else if $digest -}}
{{- printf "%s@%s" $repositoryName $digest -}}
{{- else -}}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}

{{/*
KubeAdaptor Image
*/}}
{{- define "skupper.kubeAdaptorImage" -}}
{{- $deprecatedImageRef := .Values.kubeAdaptorImage -}}
{{- $repositoryName := .Values.kubeAdaptor.repository -}}
{{- $tag := default .Chart.AppVersion .Values.kubeAdaptor.tag -}}
{{- $digest := .Values.kubeAdaptor.digest -}}
{{- if $deprecatedImageRef -}}
{{- $deprecatedImageRef  -}}
{{- else if $digest -}}
{{- printf "%s@%s" $repositoryName $digest -}}
{{- else -}}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}

{{/*
Router Image
*/}}
{{- define "skupper.routerImage" -}}
{{- $deprecatedImageRef := .Values.routerImage -}}
{{- $repositoryName := .Values.skupperRouter.repository -}}
{{- $tag := default .Chart.AppVersion .Values.skupperRouter.tag -}}
{{- $digest := .Values.skupperRouter.digest -}}
{{- if $deprecatedImageRef -}}
{{- $deprecatedImageRef  -}}
{{- else if $digest -}}
{{- printf "%s@%s" $repositoryName $digest -}}
{{- else -}}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}
