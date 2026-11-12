#!/usr/bin/env python3
"""Capture the card grid with pictures, from a data directory this script builds itself.

    python3 tools/cardmedia.py            # writes docs/screenshots/criteria/card-media-1180.png
    python3 tools/cardmedia.py --open     # also leaves the window open

Why a script rather than a screenshot of whatever the app happens to have
-----------------------------------------------------------------------
The reference picture is a full feed: every card carries a picture. This build's demo database has
eight bookmarks with no media at all, so its captures show eight text-only cards, and comparing
those against the reference says the cards are "cramped" when what is missing is the media. Round
452 built this so the comparison has something to be about.

How the pictures get there without a network
--------------------------------------------
media.MediaThumbnailURL needs a photo URL, and the app decodes what its disk cache holds: a file at
<data dir>/media-cache/<sha256(url)>.bin. So this script chooses the URLs, writes matching PNGs
into that cache under their digests, and the app finds them on the first frame. Nothing is fetched,
so the capture is deterministic and works offline.

The database is built from the app's own demo database rather than invented here: the schema, the
row identity and the eight authors all come from it, and this only sets media_json on each row.
"""

import argparse
import hashlib
import os
import shutil
import sqlite3
import struct
import subprocess
import sys
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
APP = ROOT / "build/darwin-arm64/X Bookmark Manager.app"
BIN = APP / "Contents/MacOS/X Bookmark Manager"
DEMO_DB = Path.home() / "Library/Application Support/X Bookmark Manager/x-master.db"
DATA = Path("/tmp/xbm-cardmedia-db")
OUT = ROOT.parent.parent / "docs/screenshots/criteria/card-media-1180.png"
WIDTH = 1180
DELAY = "6s"

# Eight grounds, far enough apart to see at a glance which picture landed on which card. Each is a
# vertical gradient, because a flat fill would not show whether the image is being scaled at all.
GROUNDS = [
    (32, 44, 68), (68, 40, 52), (36, 62, 56), (62, 56, 34),
    (44, 38, 66), (30, 56, 68), (66, 44, 40), (40, 60, 42),
]


def png(width, height, rgb):
    """A minimal RGB PNG: a vertical gradient from the given ground to near-white."""
    r0, g0, b0 = rgb
    rows = []
    for y in range(height):
        t = y / max(1, height - 1)
        r = int(r0 + (245 - r0) * t)
        g = int(g0 + (245 - g0) * t)
        b = int(b0 + (245 - b0) * t)
        row = b"\x00" + bytes([r, g, b]) * width
        rows.append(row)
    raw = b"".join(rows)

    def chunk(kind, payload):
        return (struct.pack(">I", len(payload)) + kind + payload +
                struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF))

    return (b"\x89PNG\r\n\x1a\n" +
            chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)) +
            chunk(b"IDAT", zlib.compress(raw, 9)) +
            chunk(b"IEND", b""))


def build_db():
    if not DEMO_DB.exists():
        print(f"cannot run: {DEMO_DB} is missing (launch the app once)", file=sys.stderr)
        return None
    DATA.mkdir(parents=True, exist_ok=True)
    db_path = DATA / "x-master.db"
    shutil.copy2(DEMO_DB, db_path)

    cache = DATA / "media-cache"
    cache.mkdir(parents=True, exist_ok=True)

    conn = sqlite3.connect(db_path)
    ids = [r[0] for r in conn.execute("SELECT tweet_id FROM bookmarks ORDER BY saved_at DESC")]
    if not ids:
        print("cannot run: the demo database has no bookmarks", file=sys.stderr)
        return None

    for i, tweet_id in enumerate(ids):
        url = f"https://xbm.invalid/media/{tweet_id}.png"
        media = f'[{{"type":"photo","url":"{url}","altText":""}}]'
        conn.execute("UPDATE bookmarks SET media_json = ? WHERE tweet_id = ?", (media, tweet_id))
        digest = hashlib.sha256(url.encode()).hexdigest()
        # 3:2, the shape a feed picture takes; the card scales it to its own height.
        (cache / f"{digest}.bin").write_bytes(png(600, 400, GROUNDS[i % len(GROUNDS)]))
    conn.commit()
    n = conn.execute("SELECT COUNT(*) FROM bookmarks WHERE media_json != '[]'").fetchone()[0]
    conn.close()
    print(f"  built {DATA} with {n} picture-bearing bookmarks")
    return DATA


def capture():
    if not BIN.exists():
        print(f"cannot run: no binary at {BIN} -- run: go tool mygo build -skip-dmg", file=sys.stderr)
        return 2
    OUT.parent.mkdir(parents=True, exist_ok=True)
    if OUT.exists():
        OUT.unlink()
    env = dict(os.environ, XBM_DATA_DIR=str(DATA), XBM_CAPTURE=str(OUT), XBM_HEADLESS="1",
               XBM_CAPTURE_DELAY=DELAY, XBM_CAPTURE_WIDTH=str(WIDTH))
    subprocess.run([str(BIN)], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["pkill", "-f", "X Bookmark Manager"], stdout=subprocess.DEVNULL,
                   stderr=subprocess.DEVNULL)
    if not OUT.exists() or OUT.stat().st_size == 0:
        print(f"  FAIL {OUT} was not written", file=sys.stderr)
        return 1
    print(f"  ok   {OUT.name}  {OUT.stat().st_size} bytes")
    return 0


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--open", action="store_true", help="leave the window open instead")
    args = parser.parse_args(argv)
    if build_db() is None:
        return 2
    if args.open:
        subprocess.run([str(BIN)], env=dict(os.environ, XBM_DATA_DIR=str(DATA)))
        return 0
    return capture()


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
