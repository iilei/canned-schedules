"""Rebuild fonts/schedule-braille-mono.woff2: the ASCII tab's font, a renamed subset of Cascadia Mono.

Run through `mise run fonts:braille`, which pins fontTools and brotli. Downloads the pinned Cascadia release, checks
its SHA-256, keeps printable ASCII, Latin-1 and the Braille Patterns block without OpenType features, and renames the
family: the SIL OFL reserves "Cascadia Code" for unmodified fonts (fonts/OFL-schedule-braille-mono.txt).
"""

import hashlib
import io
import os
import sys
import tempfile
import urllib.request
import zipfile

from fontTools import subset
from fontTools.ttLib import TTFont

RELEASE = "2407.24"
URL = f"https://github.com/microsoft/cascadia-code/releases/download/v{RELEASE}/CascadiaCode-{RELEASE}.zip"
ZIP_SHA256 = "e67a68ee3386db63f48b9054bd196ea752bc6a4ebb4df35adce6733da50c8474"
MEMBER = "ttf/static/CascadiaMono-Regular.ttf"
UNICODES = "U+0020-007E,U+00A0-00FF,U+2800-28FF"

FAMILY = "Schedule Braille Mono"
FULL = "Schedule Braille Mono Regular"
POSTSCRIPT = "ScheduleBrailleMono-Regular"
# head.modified of the committed font (2026-10-24 16:18:54 UTC). fontTools would stamp the current time on every
# save; a fixed value keeps the output byte-identical with the pinned fontTools and brotli.
MODIFIED = 3875703534

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fonts", "schedule-braille-mono.woff2")


def main():
    with urllib.request.urlopen(URL) as resp:  # noqa: S310 (fixed https URL)
        data = resp.read()
    digest = hashlib.sha256(data).hexdigest()
    if digest != ZIP_SHA256:
        sys.exit(f"checksum mismatch for {URL}: got {digest}, want {ZIP_SHA256}")

    with tempfile.TemporaryDirectory() as tmp:
        src, sub = os.path.join(tmp, "CascadiaMono-Regular.ttf"), os.path.join(tmp, "subset.woff2")
        with zipfile.ZipFile(io.BytesIO(data)) as z, open(src, "wb") as f:
            f.write(z.read(MEMBER))
        # pyftsubset's own entry point with the arguments the font was first built with: the library defaults
        # differ from the command line's, and only these reproduce the committed file.
        subset.main([src, f"--unicodes={UNICODES}", "--layout-features=", "--flavor=woff2", f"--output-file={sub}"])
        rename(sub, OUT)

    cmap = TTFont(OUT).getBestCmap()
    braille = sum(1 for c in cmap if 0x2800 <= c <= 0x28FF)
    print(f"wrote {os.path.relpath(OUT)}: {len(cmap)} code points, {braille} braille")


def rename(src, dst):
    """Gives the subset its own family name and the fixed timestamp."""
    font = TTFont(src, recalcTimestamp=False)
    names = font["name"]
    for rec in list(names.names):
        if rec.nameID in (1, 16):
            names.setName(FAMILY, rec.nameID, rec.platformID, rec.platEncID, rec.langID)
        elif rec.nameID in (3, 4):
            names.setName(FULL, rec.nameID, rec.platformID, rec.platEncID, rec.langID)
        elif rec.nameID == 6:
            names.setName(POSTSCRIPT, rec.nameID, rec.platformID, rec.platEncID, rec.langID)

    font["head"].modified = MODIFIED
    font.flavor = "woff2"
    font.save(dst)


if __name__ == "__main__":
    main()
