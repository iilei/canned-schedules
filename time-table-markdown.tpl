{{- $embedSVG := eq (env.Getenv "usage_svg" "false") "true" -}}
# Capacity Schedules

## Configured Schedules

{{ range $stage, $_ := .capacity_schedules }}- `{{ $stage }}`
{{ end }}

## Weekly Schedule Drawings

Configured schedule patterns, rendered in 30-minute slots.

{{ range $stage, $schedule := .capacity_schedules }}### {{ $stage }}

{{ if $embedSVG }}
![{{ $stage }} capacity schedule](schedule-{{ $stage }}.svg)
{{ else }}
~~~text
{{ $singleScheduleData := dict
	"capacity_schedules" (dict $stage $schedule)
	"capacity_legend" $.capacity_legend
	"aws_cron" (dict "schedules" (dict $stage (index $.aws_cron.schedules $stage)))
	"time_scale_ruler" $.time_scale_ruler
	"time_scale_annotations" $.time_scale_annotations
}}
{{ tmpl.Exec "time-table" $singleScheduleData }}
~~~
{{ end }}

{{ end }}
