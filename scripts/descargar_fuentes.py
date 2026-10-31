"""Descarga todas las fuentes del API a fuentes/ tal cual (bytes originales)."""
import json, os, sys, time, urllib.request, urllib.error
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BASE = os.environ.get("FUENTES_URL", "https://reto-pii-fuentes-production.up.railway.app/v1")
TOKEN = os.environ.get("TOKEN") or [l.split("=",1)[1].strip() for l in (ROOT/".env").read_text().splitlines() if l.startswith("TOKEN=")][0]
OUT = Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT / "fuentes"

def get(path):
    req = urllib.request.Request(BASE + path, headers={"Authorization": f"Bearer {TOKEN}"})
    for i in range(5):
        try:
            with urllib.request.urlopen(req, timeout=60) as r:
                return r.read(), dict(r.headers)
        except urllib.error.HTTPError as e:
            if e.code == 429: time.sleep(2 ** i); continue
            raise
    raise RuntimeError(path)

def save(rel, data):
    p = OUT / rel; p.parent.mkdir(parents=True, exist_ok=True); p.write_bytes(data)

def main():
    for rel, path in [("chats/channels.json", "/chats/channels"), ("chats/users.json", "/chats/users"),
                      ("docs/index.json", "/docs"), ("tables/index.json", "/tables"),
                      ("record_index.json", "/record_index"), ("tests_public.json", "/tests/public")]:
        save(rel, get(path)[0])
    for ch in json.loads((OUT/"chats/channels.json").read_text()):
        name = ch["name"]
        fechas = json.loads(get(f"/chats/{name}")[0])
        for f in fechas:
            save(f"chats/{name}/{f}.json", get(f"/chats/{name}/{f}")[0])
    for d in json.loads((OUT/"docs/index.json").read_text()):
        save(f"docs/{d['filename']}", get(f"/docs/{d['id']}")[0])
    for t in json.loads((OUT/"tables/index.json").read_text()):
        save(f"tables/{t['name']}", get(f"/tables/{t['name']}")[0])
    print("ok", OUT)

main()
