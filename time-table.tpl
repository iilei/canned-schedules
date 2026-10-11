{{- $padding := tmpl.Exec "time-scale-padding" -}}
{{- $showRuler := eq (env.Getenv "usage_ruler" "false") "true" -}}
{{- $showLegend := eq (env.Getenv "usage_legend" "false") "true" -}}
{{- $days := coll.Slice "MON" "TUE" "WED" "THU" "FRI" "SAT" "SUN" -}}
{{- $schedules := .capacity_schedules -}}
{{- if env.HasEnv "SCHEDULE_NAME" -}}
  {{- $stage := env.Getenv "SCHEDULE_NAME" -}}
  {{- $schedules = dict $stage (index $schedules $stage) -}}
{{- end -}}
{{- range $stage, $schedule := $schedules -}}
  {{- printf "[%s]\n" $stage -}}
  {{- if $showRuler -}}
    {{- tmpl.Exec "time-scale" $ | strings.Indent 4 | printf "%s" -}}
  {{- end -}}
  {{- range $day := $days -}}
    {{- if coll.Has $schedule $day -}}
      {{- printf "%s %s%s\n" $day $padding (index $schedule $day) -}}
    {{- end -}}
  {{- end -}}
  {{- range $day, $slots := $schedule -}}
    {{- if not (coll.Has $days $day) -}}
      {{- printf "%s %s%s\n" $day $padding $slots -}}
    {{- end -}}
  {{- end -}}
  {{- $transitions := index $.aws_cron.schedules $stage | default (coll.Slice) -}}
  {{- if gt (len $transitions) 0 -}}
    {{- printf "AWS Cron:\n" -}}
    {{- range $transitions -}}
      {{- printf "  %s  %s -> %s\n" .cron .from_tier .to_tier -}}
    {{- end -}}
  {{- else -}}
    {{- printf "AWS Cron: no tier transitions\n" -}}
  {{- end -}}
  {{- printf "\n" -}}
{{- end -}}
{{- if $showLegend -}}
  {{- printf "Legend:\n" -}}
  {{- range .capacity_legend -}}
    {{- printf "  %s %s\n" .symbol .tier -}}
  {{- end -}}
{{- end -}}
