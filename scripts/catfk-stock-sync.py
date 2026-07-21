#!/usr/bin/env python3
"""catfk-stock-sync.py — keep CatFK goods stocked with new-api redemption codes.

For each goods: query CatFK stock count; when below threshold, generate a batch
of redemption codes in new-api (admin API) and top up CatFK stock via catfk-cli.

Goods map (from PAR's catfkPlans.ts):
  plan goods  — deliver subscription-type codes (plan grant on redeem)
  topup goods — deliver quota codes at ¥1 = $2 (¥20/50/100/200 -> $40/100/200/400)

Usage:
  python3 catfk-stock-sync.py [--once] [--loop SECONDS] [--confirm]
  default is dry-run for writes; --confirm actually creates codes and tops up stock.

Config (env):
  NEW_API_BASE       default http://localhost:3000
  NEW_API_ADMIN_TOKEN  new-api admin PAT (default: reads /tmp/newapi-pat.txt)
  CATFK_CLI          default /Users/bytedance/my-documents/tools/catfk-cli/bin/catfk
  STOCK_LOW_WATER    default 5
  STOCK_BATCH        default 10
"""
import json
import os
import subprocess
import sys
import tempfile
import time
import urllib.request

QUOTA_PER_USD = 500_000

# goods_key -> ("plan", new-api plan_id) or ("quota", quota_units)
GOODS = {
    "vk898s": ("plan", 2),   # Mini ¥59
    "e0b3y5": ("plan", 3),   # Pro mini ¥119
    "r07y8g": ("plan", 1),   # Pro x1 ¥199
    "uhwx0f": ("plan", 4),   # Pro x2 ¥329
    "bx9j3s": ("plan", 5),   # Pro x3 ¥499
    "r5ufqm": ("quota", 40 * QUOTA_PER_USD),   # ¥20 -> $40
    "ot5e6z": ("quota", 100 * QUOTA_PER_USD),  # ¥50 -> $100
    "jyq5ae": ("quota", 200 * QUOTA_PER_USD),  # ¥100 -> $200
    "paibsa": ("quota", 400 * QUOTA_PER_USD),  # ¥200 -> $400
}

NEW_API_BASE = os.environ.get("NEW_API_BASE", "http://localhost:3000")
CATFK_CLI = os.environ.get("CATFK_CLI", "/Users/bytedance/my-documents/tools/catfk-cli/bin/catfk")
LOW_WATER = int(os.environ.get("STOCK_LOW_WATER", "5"))
BATCH = int(os.environ.get("STOCK_BATCH", "10"))


def admin_token():
    tok = os.environ.get("NEW_API_ADMIN_TOKEN")
    if tok:
        return tok.strip()
    return open("/tmp/newapi-pat.txt").read().strip()


def api(path, payload=None):
    body = json.dumps(payload).encode() if payload is not None else None
    req = urllib.request.Request(NEW_API_BASE + path, data=body, headers={
        "Authorization": f"Bearer {admin_token()}", "Content-Type": "application/json"},
        method="POST" if payload is not None else "GET")
    return json.loads(urllib.request.urlopen(req).read().decode())


def catfk(*args):
    r = subprocess.run([CATFK_CLI, *args, "--json"], capture_output=True, text=True)
    try:
        return json.loads(r.stdout)
    except json.JSONDecodeError:
        return {"_raw": r.stdout, "_err": r.stderr, "_rc": r.returncode}


def goods_id_map():
    """goods_key -> numeric goods id, for stock operations."""
    r = catfk("goods", "list")
    items = r.get("data") or r.get("goods") or []
    if isinstance(items, dict):
        items = items.get("items", [])
    return {g.get("goods_key"): g.get("id") for g in items}


def stock_available(goods_id):
    r = catfk("stock", "list", str(goods_id))
    items = r.get("list") or r.get("data") or []
    # catfk card status: 0 = available(unsold), 1 = sold
    return sum(1 for c in items if c.get("status") == 0)


def create_codes(kind, value, count):
    """Create `count` redemption codes in new-api, return the key list."""
    payload = {"name": "catfk-sync", "count": count, "expired_time": 0}
    if kind == "plan":
        payload.update({"redemption_type": "subscription", "subscription_plan_id": value, "quota": 0})
    else:
        payload.update({"redemption_type": "quota", "quota": value})
    r = api("/api/redemption/", payload)
    if not r.get("success"):
        raise RuntimeError(f"create codes failed: {r}")
    return r.get("data") or []


def sync_once(confirm):
    id_map = goods_id_map()
    report = []
    for goods_key, (kind, value) in GOODS.items():
        gid = id_map.get(goods_key)
        if not gid:
            report.append(f"{goods_key}: SKIP (goods not found)")
            continue
        avail = stock_available(gid)
        if avail >= LOW_WATER:
            report.append(f"{goods_key}({kind}:{value}): stock={avail} ok")
            continue
        need = BATCH
        report.append(f"{goods_key}({kind}:{value}): stock={avail} < {LOW_WATER}, top up {need}")
        if not confirm:
            continue
        keys = create_codes(kind, value, need)
        if not keys:
            report.append(f"  !! no keys created")
            continue
        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as f:
            f.write("\n".join(keys) + "\n")
            cards_path = f.name
        r = catfk("stock", "add", str(gid), "--cards", cards_path, "--confirm")
        ok = r.get("ok", r.get("success"))
        report.append(f"  stock add -> {ok} ({len(keys)} cards)")
        os.unlink(cards_path)
    return report


def main():
    args = set(sys.argv[1:])
    confirm = "--confirm" in args
    loop_s = 0
    if "--loop" in sys.argv:
        i = sys.argv.index("--loop")
        loop_s = int(sys.argv[i + 1])
    while True:
        for line in sync_once(confirm):
            print(line, flush=True)
        if not loop_s:
            break
        time.sleep(loop_s)


if __name__ == "__main__":
    main()
