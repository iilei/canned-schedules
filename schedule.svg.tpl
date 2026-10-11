{{- $data := ds "data" -}}
{{- $fontFamily := "Ubuntu Mono, 'Courier New', Courier, monospace" -}}
{{- $fontFamilyHeading := "Ubuntu Mono, 'Courier New', Courier, monospace" -}}
{{- $scheduleName := env.Getenv "SCHEDULE_NAME" -}}
{{- $schedule := index $data.capacity_schedules $scheduleName -}}
{{- $events := index $data.aws_cron.schedules $scheduleName | default (coll.Slice) -}}
{{- $showRuler := eq (env.Getenv "usage_ruler" "false") "true" -}}
{{- $rulerOffset := 0 -}}
{{- if $showRuler -}}
  {{- $offsetText := "0" -}}
  {{- if env.HasEnv "usage_offset" -}}
    {{- $offsetText = env.Getenv "usage_offset" -}}
  {{- else if env.HasEnv "TIME_SCALE_OFFSET" -}}
    {{- $offsetText = env.Getenv "TIME_SCALE_OFFSET" -}}
  {{- end -}}
  {{- if not (regexp.Match "^[0-9]+$" $offsetText) -}}
    {{- test.Fail "offset (argument or TIME_SCALE_OFFSET) must be a non-negative integer" -}}
  {{- end -}}
  {{- $rulerOffset = conv.Atoi $offsetText -}}
{{- end -}}
{{- $days := coll.Slice "MON" "TUE" "WED" "THU" "FRI" "SAT" "SUN" -}}
{{- $slotCount := len $data.time_brackets -}}
{{- $blockW := 12 -}}
{{- $blockH := 24 -}}
{{- $labelW := 40 -}}
{{- $headerH := 30 -}}
{{- $stageTitleH := 28 -}}
{{- $stageY := add $headerH $stageTitleH -}}
{{- $stageH := 24 -}}
{{- if $showRuler -}}
  {{- $stageH = 44 -}}
{{- end -}}
{{- $dayH := 32 -}}
{{- $scheduleH := add $stageH (mul (len $days) $dayH) -}}
{{- $cronLineH := 18 -}}
{{- $scheduleW := add $labelW (mul $slotCount $blockW) -}}
{{- $columnGap := 28 -}}
{{- $cronW := 315 -}}
{{- $cronX := add $scheduleW $columnGap -}}
{{- $cronTextX := add $cronX 6 -}}
{{- $transitionDividerX := add $cronX 165 -}}
{{- $transitionX := add $transitionDividerX 6 -}}
{{- $legendPaddingRight := 32 -}}
{{- $totalW := add $cronX $cronW -}}
{{- $cronLineCount := add 1 (len $events) -}}
{{- if eq (len $events) 0 -}}
  {{- $cronLineCount = 2 -}}
{{- end -}}
{{- $cronH := add $stageH (add 1 (mul $cronLineCount $cronLineH)) -}}
{{- $contentH := $scheduleH -}}
{{- if gt $cronH $contentH -}}
  {{- $contentH = $cronH -}}
{{- end -}}
{{- $legendH := 44 -}}
{{- $totalH := add (add $stageY $contentH) $legendH -}}
<svg xmlns="http://www.w3.org/2000/svg" width="{{ $totalW }}" height="{{ $totalH }}" viewBox="0 0 {{ $totalW }} {{ $totalH }}" style="color-scheme: light dark">
  <style>
    .schedule-background { fill: {{ $data.schedule_theme.light.background }}; }
    .schedule-label { fill: {{ $data.schedule_theme.light.label }}; }
    .schedule-divider { stroke: {{ $data.schedule_theme.light.label }}; stroke-opacity: 0.3; }
    .schedule-tier { fill: var(--tier-color-light); }
    {{- range $legendIndex, $legend := $data.capacity_legend -}}
      {{- $tier := index $data.capacity_tiers $legend.tier -}}
    .schedule-tier-{{ $legendIndex }} { --tier-color-light: {{ $tier.color.light }}; --tier-color-dark: {{ $tier.color.dark }}; }
    {{- end }}
    @media (prefers-color-scheme: dark) {
      .schedule-background { fill: {{ $data.schedule_theme.dark.background }}; }
      .schedule-label { fill: {{ $data.schedule_theme.dark.label }}; }
      .schedule-divider { stroke: {{ $data.schedule_theme.dark.label }}; stroke-opacity: 0.3; }
      .schedule-tier { fill: var(--tier-color-dark); }
    }
  </style>
  <rect class="schedule-background" width="100%" height="100%"/>
  <line class="schedule-divider" x1="{{ add $scheduleW (div $columnGap 2) }}" y1="{{ $stageY }}" x2="{{ add $scheduleW (div $columnGap 2) }}" y2="{{ add $stageY $contentH }}"/>
  <text class="schedule-label" x="{{ $labelW }}" y="{{ add $headerH 19 }}" font-size="18" font-family="{{ $fontFamilyHeading }}" font-style="italic" font-weight="bold">{{ $scheduleName }}</text>
  {{- if $showRuler }}
  <g class="schedule-ruler">
  {{- range $slot, $marker := strings.Split "" $data.time_scale_ruler }}
  {{- if ne $marker " " }}
  {{- $tickX := add $labelW (mul $slot $blockW) }}
  {{- $tickH := 4 }}
  {{- if eq $marker "|" }}
    {{- $tickH = 8 }}
  {{- else if eq $marker "║" }}
    {{- $tickH = 12 }}
  {{- end }}
  <line class="schedule-divider" x1="{{ $tickX }}" y1="{{ sub (add $stageY 36) $tickH }}" x2="{{ $tickX }}" y2="{{ add $stageY 36 }}"/>
  {{- if or (eq $marker "║") (eq $marker "|") }}
  <text class="schedule-label" x="{{ $tickX }}" dx="{{ sub 0 $rulerOffset }}ch" y="{{ add $stageY 17 }}" font-size="12" font-family="{{ $fontFamily }}" text-anchor="start">{{ index $data.time_scale_annotations $slot }}</text>
  {{- end }}
  {{- end }}
  {{- end }}
  </g>
  {{- end }}
  {{- range $dayIndex, $dayName := $days }}
  {{- $row := index $schedule $dayName }}
  {{- $y := add $stageY (add $stageH (mul $dayIndex $dayH)) }}
  <text class="schedule-label" x="4" y="{{ add $y 18 }}" font-size="14" font-family="{{ $fontFamily }}">{{ $dayName }}</text>
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
  {{- range $legendIndex, $legend := $data.capacity_legend }}
  {{- if eq $legend.symbol $symbol }}
  <rect class="schedule-tier schedule-tier-{{ $legendIndex }}" x="{{ add $labelW (mul $i $blockW) }}" y="{{ $y }}" width="{{ $blockW }}" height="{{ $blockH }}" rx="1"><title>{{ if $activeEvent }}Schedule: cron({{ $activeEvent.cron }})&#10;Set tier: {{ $activeEvent.to_tier }}{{ else }}Schedule: no tier transitions&#10;Set tier: {{ $legend.tier }}{{ end }}</title></rect>
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- $cronY := add $stageY $stageH }}
  <text class="schedule-label" x="{{ $cronTextX }}" y="{{ add $cronY 18 }}" font-size="14" font-family="{{ $fontFamily }}">AWS Cron Expression</text>
  <text class="schedule-label" x="{{ $transitionX }}" y="{{ add $cronY 18 }}" font-size="14" font-family="{{ $fontFamily }}">Transition</text>
  {{- if eq (len $events) 0 }}
  <text class="schedule-label" x="{{ $cronTextX }}" y="{{ add (add $cronY $cronLineH) 18 }}" font-size="12" font-family="{{ $fontFamily }}">No tier transitions</text>
  {{- else }}
  {{- range $eventIndex, $event := $events }}
  <text class="schedule-label" x="{{ $cronTextX }}" y="{{ add (add $cronY (mul (add $eventIndex 1) $cronLineH)) 18 }}" font-size="12" font-family="{{ $fontFamily }}">{{ $event.cron }}</text>
  <text class="schedule-label" x="{{ $transitionX }}" y="{{ add (add $cronY (mul (add $eventIndex 1) $cronLineH)) 18 }}" font-size="12" font-family="{{ $fontFamily }}">{{ $event.from_tier }} -&gt; {{ $event.to_tier }}</text>
  {{- end }}
  {{- end }}
  {{- $legendY := add $stageY $contentH }}
  {{- $legendX := $labelW }}
  <line class="schedule-divider" x1="{{ $labelW }}" y1="{{ add $legendY 4 }}" x2="{{ sub $totalW 4 }}" y2="{{ add $legendY 4 }}"/>
  {{- range $legendIndex, $legend := $data.capacity_legend }}
  <rect class="schedule-tier schedule-tier-{{ $legendIndex }}" x="{{ $legendX }}" y="{{ add $legendY 13 }}" width="20" height="16" rx="1"/>
  <text class="schedule-label" x="{{ add $legendX 26 }}" y="{{ add $legendY 26 }}" font-size="12" font-family="{{ $fontFamily }}">{{ $legend.tier }}</text>
  {{- $legendX = add $legendX (add 26 (add (mul (len $legend.tier) 8) $legendPaddingRight)) }}
  {{- end }}
</svg>
