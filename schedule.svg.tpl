{{- $data := ds "data" -}}
{{- $days := coll.Slice "MON" "TUE" "WED" "THU" "FRI" "SAT" "SUN" -}}
{{- $slotCount := len $data.time_brackets -}}
{{- $blockW := 12 -}}
{{- $blockH := 24 -}}
{{- $labelW := 40 -}}
{{- $headerH := 30 -}}
{{- $stageH := 24 -}}
{{- $dayH := 32 -}}
{{- $scheduleH := add $stageH (mul (len $days) $dayH) -}}
{{- $totalW := add $labelW (mul $slotCount $blockW) -}}
{{- $totalH := add $headerH (mul (len $data.capacity_schedules) $scheduleH) -}}
{{- $stageIndex := 0 -}}
<svg xmlns="http://www.w3.org/2000/svg" width="{{ $totalW }}" height="{{ $totalH }}" viewBox="0 0 {{ $totalW }} {{ $totalH }}">
  <rect width="100%" height="100%" fill="{{ $data.schedule_theme.background }}"/>
  {{- range $stageName, $schedule := $data.capacity_schedules }}
  {{- $stageY := add $headerH (mul $stageIndex $scheduleH) }}
  <text x="4" y="{{ add $stageY 17 }}" fill="{{ $data.schedule_theme.label }}" font-size="14" font-family="Ubuntu Mono, 'Courier New', Courier, monospace">{{ $stageName }}</text>
  {{- range $dayIndex, $dayName := $days }}
  {{- $row := index $schedule $dayName }}
  {{- $y := add $stageY (add $stageH (mul $dayIndex $dayH)) }}
  <text x="4" y="{{ add $y 18 }}" fill="{{ $data.schedule_theme.label }}" font-size="14" font-family="Ubuntu Mono, 'Courier New', Courier, monospace">{{ $dayName }}</text>
  {{- range $i, $symbol := strings.Split "" $row }}
  {{- range $legend := $data.capacity_legend }}
  {{- if eq $legend.symbol $symbol }}
  {{- $tier := index $data.capacity_tiers $legend.tier }}
  <rect x="{{ add $labelW (mul $i $blockW) }}" y="{{ $y }}" width="{{ $blockW }}" height="{{ $blockH }}" fill="{{ $tier.color }}" rx="1"/>
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- $stageIndex = add $stageIndex 1 }}
  {{- end }}
</svg>
