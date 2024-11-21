{{- define "name-client" -}}
{{ printf "%s-client" $.Chart.Name | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "name-server" -}}
{{ printf "%s-server" $.Chart.Name | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}
