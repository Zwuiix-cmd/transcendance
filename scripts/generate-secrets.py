#!/usr/bin/env python3
"""Print fresh development secrets without writing them to disk."""

from secrets import token_hex


print(f"API_KEY={token_hex(32)}")
print(f"INTERNAL_CDN_API_KEY={token_hex(32)}")
print(f"GAME_SERVICE_KEY={token_hex(32)}")
