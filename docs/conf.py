# The documentation site: Sphinx with Furo and MyST, plus the canned-schedules Sphinx kit, which mise run docs:build
# generates with this checkout's CLI into _ext/ (the same kit the playground offers for download). Build with
# mise run docs:build; the Pages site puts the playground under playground/.
import os
import sys

project = "canned-schedules"
html_title = "canned-schedules"
html_theme = "furo"
html_baseurl = "https://iilei.github.io/canned-schedules/"
html_static_path = ["_static"]
html_js_files = ["legacy-links.js"]
html_theme_options = {
    "source_repository": "https://github.com/iilei/canned-schedules/",
    "source_branch": "master",
    "source_directory": "docs/",
}

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
