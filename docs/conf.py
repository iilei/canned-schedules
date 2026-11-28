# The documentation site: Sphinx with Furo and MyST, plus the canned-schedules Sphinx kit, which mise run docs:build
# generates with this checkout's CLI into _ext/ (the same kit the playground offers for download). Build with
# mise run docs:build; the Pages site puts the playground under playground/. Fonts, accents and highlighting match
# the companion's docs (canned-schedules-companion docs/conf.py).
import os
import sys

project = "canned-schedules"
html_title = "canned-schedules"
html_theme = "furo"
html_baseurl = "https://iilei.github.io/canned-schedules/"
html_theme_options = {
    "source_repository": "https://github.com/iilei/canned-schedules/",
    "source_branch": "master",
    "source_directory": "docs/",
    # Served from _static (fonts.css), with the system fonts as fallback. Furo uses these in dark mode too.
    "light_css_variables": {
        "font-stack": 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif',
        "font-stack--headings": '"Source Serif 4", Georgia, "Times New Roman", serif',
        "font-stack--monospace": '"Noto Sans Mono", SFMono-Regular, Menlo, Consolas, monospace',
        # The playground's accents (web/style.css): teal for links and navigation, violet for visited links, the
        # hover underline, the current page and "important".
        "color-brand-primary": "#0b7a68",
        "color-brand-content": "#0b7a68",
        "color-brand-visited": "#8250df",
        "color-link-underline--hover": "#8250df",
        "color-toc-item-text--active": "#8250df",
        "color-highlighted-background": "#e2f4ef",
        "color-admonition-title--important": "#8250df",
        "color-admonition-title-background--important": "rgba(130, 80, 223, 0.1)",
        "color-accent-current": "#8250df",
    },
    "dark_css_variables": {
        "color-brand-primary": "#5ad1b8",
        "color-brand-content": "#5ad1b8",
        "color-brand-visited": "#c4a5ff",
        "color-link-underline--hover": "#c4a5ff",
        "color-toc-item-text--active": "#c4a5ff",
        "color-highlighted-background": "#15302b",
        "color-admonition-title--important": "#c4a5ff",
        "color-admonition-title-background--important": "rgba(196, 165, 255, 0.12)",
        "color-accent-current": "#c4a5ff",
    },
}
html_static_path = ["_static"]
html_js_files = ["legacy-links.js"]
templates_path = ["_templates"]  # base.html: font preloads
html_css_files = ["fonts.css", "theme.css"]
# Both from the a11y syntax highlighting project (Furo's default dark style, native, is unrelated to its light one).
pygments_style = "a11y-light"
pygments_dark_style = "a11y-dark"

extensions = ["myst_parser"]
# Anchors for headings down to ###, named like GitHub's, so links such as syntax.md#time-and-weekday-rules work.
myst_heading_anchors = 3
myst_enable_extensions = ["colon_fence"]

# The Sphinx kit, as a team would install it: the extension loads myst_parser, colon_fence and the styles of the
# schedule admonitions (examples/ are its MyST output).
sys.path.append(os.path.abspath("_ext"))
extensions += ["canned_schedules"]

exclude_patterns = ["_build", "requirements.*", "README.md"]
# Pygments' HCL lexer gives up on Terraform interpolation and falls back to plain highlighting; that is fine.
suppress_warnings = ["myst.header", "misc.highlighting_failure"]
