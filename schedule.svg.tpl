{{- $data := ds "data" -}}
{{- $fontFamily := "Ubuntu Mono, 'Courier New', Courier, monospace" -}}
{{- $stageName := env.Getenv "SCHEDULE_NAME" -}}
{{- $schedule := index $data.capacity_schedules $stageName -}}
{{- $events := index $data.aws_cron.schedules $stageName | default (coll.Slice) -}}
{{- $days := coll.Slice "MON" "TUE" "WED" "THU" "FRI" "SAT" "SUN" -}}
{{- $slotCount := len $data.time_brackets -}}
{{- $blockW := 12 -}}
{{- $blockH := 24 -}}
{{- $labelW := 40 -}}
{{- $headerH := 30 -}}
{{- $stageH := 24 -}}
{{- $dayH := 32 -}}
{{- $scheduleH := add $stageH (mul (len $days) $dayH) -}}
{{- $cronLineH := 18 -}}
{{- $totalW := add $labelW (mul $slotCount $blockW) -}}
{{- $cronLineCount := add 1 (len $events) -}}
{{- if eq (len $events) 0 -}}
  {{- $cronLineCount = 2 -}}
{{- end -}}
{{- $totalH := add (add $headerH $scheduleH) (mul $cronLineCount $cronLineH) -}}
<svg xmlns="http://www.w3.org/2000/svg" width="{{ $totalW }}" height="{{ $totalH }}" viewBox="0 0 {{ $totalW }} {{ $totalH }}">
  <rect width="100%" height="100%" fill="{{ $data.schedule_theme.background }}"/>
  {{- $stageY := $headerH }}
  <text x="4" y="{{ add $stageY 17 }}" fill="{{ $data.schedule_theme.label }}" font-size="14" font-family="{{ $fontFamily }}">{{ $stageName }}</text>
  {{- range $dayIndex, $dayName := $days }}
  {{- $row := index $schedule $dayName }}
  {{- $y := add $stageY (add $stageH (mul $dayIndex $dayH)) }}
  <text x="4" y="{{ add $y 18 }}" fill="{{ $data.schedule_theme.label }}" font-size="14" font-family="{{ $fontFamily }}">{{ $dayName }}</text>
  {{- range $i, $symbol := strings.Split "" $row }}
  {{- $currentMinute := add (mul $dayIndex 1440) (index $data.time_brackets $i) -}}
  {{- $weekMinutes := mul (len $days) 1440 -}}
  {{- $activeOccurrence := sub 0 $weekMinutes -}}
  {{- $activeEvent := dict -}}
  {{- range $event := $events -}}
    {{- range $eventDayIndex, $eventDay := $days -}}
      {{- if coll.Has $event.days $eventDay -}}
        {{- $occurrence := add (mul $eventDayIndex 1440) (add (mul $event.hour 60) $event.minute) -}}
        {{- if gt $occurrence $currentMinute -}}
          {{- $occurrence = sub $occurrence $weekMinutes -}}
        {{- end -}}
        {{- if gt $occurrence $activeOccurrence -}}
          {{- $activeOccurrence = $occurrence -}}
          {{- $activeEvent = $event -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- range $legend := $data.capacity_legend }}
  {{- if eq $legend.symbol $symbol }}
  {{- $tier := index $data.capacity_tiers $legend.tier }}
  <rect x="{{ add $labelW (mul $i $blockW) }}" y="{{ $y }}" width="{{ $blockW }}" height="{{ $blockH }}" fill="{{ $tier.color }}" rx="1"><title>{{ if $activeEvent }}Schedule: cron({{ $activeEvent.cron }})&#10;Set tier: {{ $activeEvent.to_tier }}{{ else }}Schedule: no tier transitions&#10;Set tier: {{ $legend.tier }}{{ end }}</title></rect>
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- $cronY := add $stageY $scheduleH }}
  <text x="4" y="{{ add $cronY 14 }}" fill="{{ $data.schedule_theme.label }}" font-size="12" font-family="{{ $fontFamily }}">AWS Cron (UTC)</text>
  {{- if eq (len $events) 0 }}
  <text x="{{ $labelW }}" y="{{ add (add $cronY $cronLineH) 14 }}" fill="{{ $data.schedule_theme.label }}" font-size="12" font-family="{{ $fontFamily }}">No tier transitions</text>
  {{- else }}
  {{- range $eventIndex, $event := $events }}
  <text x="{{ $labelW }}" y="{{ add (add $cronY (mul (add $eventIndex 1) $cronLineH)) 14 }}" fill="{{ $data.schedule_theme.label }}" font-size="12" font-family="{{ $fontFamily }}">{{ $event.cron }}  {{ $event.from_tier }} -&gt; {{ $event.to_tier }}</text>
  {{- end }}
  {{- end }}
</svg>
