# Previews

```sh
./canned-schedules --output ascii schedule.md
```

```text
Office Hours - Weekdays only

    00:00                   06:00                   12:00                   18:00
    + : | : | : | : | : | : + : | : | : | : | : | : + : | : | : | : | : | : + : | : | : | : | : | :
MON ................................========================================........................
SAT ................................................................................................

    . Off (base)   : Minimum   = Medium   # Maximum
```

(Rows for TUE–FRI and SUN omitted here.) On a terminal each cell gets the dark theme's state colour as its background;
`--color never` turns that off, `--color always` forces it (also into files).

**Symbols.** By default the preview is pure ASCII (`.` `:` `=` `#` by state order), so it lines up in every terminal,
log and email. `--ascii-symbols braille` switches to `⠀` `⣀` `⣤` `⣶` `⣿`, a denser look that is still single-width
everywhere (unlike block characters such as `▒`, which CJK terminals draw double-width). `--ascii-symbols styles`
uses each configured style's own `symbol`. The flag is shorthand for
`--set-config-at representation.ascii.symbols`.

**Downsampling.** Previews show buckets of 15 minutes by default, so a day fits into 96 columns (100 with the day
label). `--preview-downsample=N` picks another bucket size (a divisor of 60), `--no-preview-downsample` shows every
minute. A bucket that contains more than one state shows the one covering most of it; the tool then warns that the
preview is approximate. Downsampling only affects previews: the JSON output stays exact. For exact previews, require
times on the bucket grid with `--sample-interval 15`.

## SVG preview

`--svg-output-file preview.svg` writes a picture of the week like the one above: one row per day, a time ruler, the
transitions with POSIX and AWS cron, and a legend with the base state marked. Hovering a block shows its state, its time
range and the cron expression that starts it. Colours come from the configuration's styles and follow the viewer's
light or dark mode. The preview is only written when the document has no errors, so a failed run never replaces a
good preview with a broken one.

The SVG comes from the built-in [`templates/schedule-svg.tmpl`](https://github.com/iilei/canned-schedules/blob/master/templates/schedule-svg.tmpl). The tool computes all
coordinates, colours and labels, so the template is plain markup that also runs with gomplate.

With `--svg-annotations`, a META section above the transition table shows the summary (moved there from beside the
title, so it sits with the rest of META) and the annotations (`x-*` keys), sorted by key as in the JSON, with a rule
between the section and the table. It is opt-in for the same reason as the source embed below: owners and links would otherwise travel with
every copy of the picture.

With `--svg-embed-source`, the SVG also carries the schedule in its `<metadata>`: the canonical form (see below) as a
`cs:source` element with the generating version as an attribute. Evaluating it regenerates the JSON, so the picture
stays traceable to its schedule. It is opt-in, because `x-*` values such as owners or issue
links then travel with every copy of the picture. Any XML tool reads it back, for example:

```sh
xmllint --xpath 'string(//*[local-name()="source"])' preview.svg > schedule.md
```

## Text form

`--output text` (or `--text-output-file <path>`) gives the schedule's **canonical form** with the ASCII preview in a
fenced block after the name:

````markdown
# Office Hours

~~~text
Office Hours - Weekdays only

    00:00                   06:00                   12:00                   18:00
    + : | : | : | : | : | : + : | : | : | : | : | : + : | : | : | : | : | : + : | : | : | : | : | :
MON ................................========================================........................
...
~~~

> [!SCHEDULE-META] Weekdays only
> - x-owner: `team-xyz`

> [!SCHEDULE-BASE] Off

> [!SCHEDULE-STATE] Medium
> - `08:00` to `18:00` MON-FRI
````

The canonical form keeps only what the syntax defines: the name, META, BASE and the STATE blocks with their ranges, in
source order. Prose, other alerts and links are dropped. Times are written `HH:MM`, weekday expressions without spaces
(`SAT, SUN` becomes `SAT,SUN`) and state names as the configuration spells them; grouping and order stay as written.
The text form is itself a valid schedule that evaluates to the same output, because the parser reads a
code block as prose.

## Paint trace

Later ranges paint over earlier ones, and a range that crosses midnight spills into the next day. To see what each
line actually does, `--verbose` prints one line per range and one for the base to stderr, linked to the source
like the error report:

```text
schedule.md:3:5: info[paint]: Minimum is the base (BASE block): 84 h not painted over
schedule.md:6:5: info[paint]: Medium MON-FRI 06:00–22:00: covers 80 h, 78 h effective; shadowed: FRI 20:00–22:00 by line 9
schedule.md:9:5: info[paint]: Maximum FRI 20:00–02:00: covers 6 h, all effective
```

`--trace-output-file <path>` writes the same trace as JSON (`trace.schema.json`): per range the minutes it covers,
where its state shows and which line paints over the rest, in exact minutes. Both are debugging aids **outside the
stability promise**: their wording and shape may change in any release, without a new `schema-version`. Build on
the [output](output.md), not on the trace. On the preflight page, the same trace drives the stripes on the week.
