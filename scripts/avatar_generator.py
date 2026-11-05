#!/usr/bin/env python3
"""Generate a small default avatar for a user.

The CDN calls this script only when a user does not have an uploaded avatar.
The output is dependency-free SVG so it can be generated in the shared project
scripts image without adding an image-processing dependency to the CDN.
"""

from __future__ import annotations

import hashlib
import random
import secrets
import sys


PALETTES = (
    ("#172554", "#60a5fa", "#dbeafe"),
    ("#312e81", "#a78bfa", "#ede9fe"),
    ("#164e63", "#22d3ee", "#cffafe"),
    ("#3f2c16", "#f5b85c", "#fff1c7"),
    ("#3f1d36", "#f472b6", "#fce7f3"),
    ("#1f2937", "#94a3b8", "#f1f5f9"),
)


def render_accent(variant: int, palette: tuple[str, str, str]) -> str:
    """Render one of the simple geometric avatar motifs."""

    _, accent, foreground = palette
    if variant == 0:
        return (
            f'<circle cx="32" cy="32" r="22" fill="{accent}" opacity=".92"/>'
            f'<circle cx="32" cy="32" r="10" fill="{foreground}" opacity=".8"/>'
        )
    if variant == 1:
        return (
            f'<path d="M-8 48 C10 20 24 12 42 22 C58 31 69 17 76 7 V72 H-8Z" '
            f'fill="{accent}" opacity=".9"/>'
            f'<circle cx="24" cy="29" r="9" fill="{foreground}"/>'
        )
    return (
        f'<rect x="12" y="12" width="40" height="40" rx="16" fill="{accent}" '
        f'opacity=".9" transform="rotate(18 32 32)"/>'
        f'<circle cx="32" cy="32" r="8" fill="{foreground}"/>'
    )


def render_avatar(owner_id: str) -> str:
    """Create a random-looking avatar that is generated once and then stored."""

    owner_hash = hashlib.sha256(owner_id.encode("utf-8")).digest()
    owner_seed = int.from_bytes(owner_hash[:8], byteorder="big")
    generator = random.Random(owner_seed ^ secrets.randbits(64))
    palette = generator.choice(PALETTES)
    rotation = generator.randrange(-12, 13)
    variant = generator.randrange(3)

    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">'
        f'<rect width="64" height="64" rx="18" fill="{palette[0]}"/>'
        f'<g transform="rotate({rotation} 32 32)">'
        f"{render_accent(variant, palette)}"
        "</g>"
        f'<path d="M16 53c5-7 11-10 16-10s11 3 16 10" fill="none" '
        f'stroke="{palette[2]}" stroke-width="3" stroke-linecap="round" opacity=".75"/>'
        "</svg>\n"
    )


def main() -> int:
    if len(sys.argv) != 2 or not sys.argv[1]:
        print("usage: avatar_generator.py <owner-id>", file=sys.stderr)
        return 2
    sys.stdout.write(render_avatar(sys.argv[1]))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
