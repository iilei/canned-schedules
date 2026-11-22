# Rendering elsewhere

GitHub and GitLab show schedule directives as plain quotes with the marker as text. Tools that follow the Obsidian
callout convention (a custom type plus a title on the marker line) render them as callouts. We recommend
[rehype-callouts](https://github.com/lin-stephanie/rehype-callouts) for remark/rehype sites such as Astro. It only
renders types it knows, so register the three kinds:

```js
import rehypeCallouts from "rehype-callouts";

rehype().use(rehypeCallouts, {
  callouts: {
    "schedule-meta": { title: "Schedule" },
    "schedule-base": { title: "Base state" },
    "schedule-state": { title: "State" },
  },
});
```

The text after the marker becomes the callout's title (the state id or the summary), and each block gets
`data-callout="schedule-state"` (lowercase) for styling with one of its themes:

```css
[data-callout="schedule-state"] {
  --rc-color-light: #2f855a;
  --rc-color-dark: #68d391;
}
```

Colours per state, display names and weekday-aware layout need the tool itself: use the [Sphinx kit](sphinx.md) or the
SVG preview for those.
