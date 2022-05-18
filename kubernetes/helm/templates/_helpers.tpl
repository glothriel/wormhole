{{- define "name-client" -}}
{{ printf "%s-client-%s" $.Chart.Name $.Release.Name | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "name-server" -}}
{{ printf "%s-server-%s" $.Chart.Name $.Release.Name | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}
