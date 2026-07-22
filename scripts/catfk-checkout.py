#!/usr/bin/env python3
"""catfk-checkout.py — CatFK anonymous checkout bridge for new-api.

Flow: user clicks Buy -> POST /checkout (with their new-api JWT) -> we create a
CatFK order and return the pay URL (Alipay or WeChat, buyer's choice) -> user
pays -> we detect payment via Pay/query, pull the delivered card secrets, and
auto-grant in new-api (subscription via admin bind, quota via admin manage
add_quota). Fully server-side: the user never has to paste a code.

Endpoints (CORS-open for local dev):
  POST /checkout        {"jwt": "...", "goods_key": "vk898s", "contact": "optional",
                         "pay": "alipay"|"wechat" (default alipay)}
                        -> {"trade_no": "...", "payurl": "..."}
  GET  /checkout/status?trade_no=...
                        -> {"status": "pending"|"paid"|"granted"|"error", ...}

A sweeper thread also polls pending orders every 30s so grants complete even if
the user closes the page. Pending orders persist in ./catfk-checkout-orders.json.

Config (env):
  NEW_API_BASE         default http://localhost:3000
  NEW_API_ADMIN_TOKEN  admin PAT (default /tmp/newapi-pat.txt)
  CATFK_CLI            default /Users/bytedance/my-documents/tools/catfk-cli/bin/catfk
  CHECKOUT_PORT        default 8390
"""
import json
import os
import subprocess
import threading
import time
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse, parse_qs

NEW_API_BASE = os.environ.get("NEW_API_BASE", "http://localhost:3000")
CATFK_CLI = os.environ.get("CATFK_CLI", "/Users/bytedance/my-documents/tools/catfk-cli/bin/catfk")
PORT = int(os.environ.get("CHECKOUT_PORT", "8390"))
ORDERS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "catfk-checkout-orders.json")
QUOTA_PER_USD = 500_000

# goods_key -> ("plan", plan_id) | ("quota", quota_units). Same map as catfk-stock-sync.
GOODS = {
    "vk898s": ("plan", 2),   # Mini ¥59
    "e0b3y5": ("plan", 3),   # Pro mini ¥119
    "r07y8g": ("plan", 1),   # Pro x1 ¥199
    "uhwx0f": ("plan", 4),   # Pro x2 ¥329
    "bx9j3s": ("plan", 5),   # Pro x3 ¥499
    "snae3x": ("plan", 6),   # Pro x4 ¥749
    "cbcg11": ("quota", 10 * QUOTA_PER_USD),   # ¥5 测试档
    "r5ufqm": ("quota", 40 * QUOTA_PER_USD),
    "ot5e6z": ("quota", 100 * QUOTA_PER_USD),
    "jyq5ae": ("quota", 200 * QUOTA_PER_USD),
    "paibsa": ("quota", 400 * QUOTA_PER_USD),
}

_lock = threading.Lock()


def admin_token():
    tok = os.environ.get("NEW_API_ADMIN_TOKEN")
    return tok.strip() if tok else open("/tmp/newapi-pat.txt").read().strip()


def load_orders():
    try:
        with _lock:
            return json.load(open(ORDERS_FILE))
    except (OSError, json.JSONDecodeError):
        return {}


def save_orders(orders):
    with _lock:
        tmp = ORDERS_FILE + ".tmp"
        with open(tmp, "w") as f:
            json.dump(orders, f, indent=1)
        os.replace(tmp, ORDERS_FILE)


def http_json(url, payload=None, bearer=None, timeout=30):
    body = json.dumps(payload).encode() if payload is not None else None
    headers = {"Content-Type": "application/json"}
    if bearer:
        headers["Authorization"] = f"Bearer {bearer}"
    req = urllib.request.Request(url, data=body, headers=headers,
                                 method="POST" if payload is not None else "GET")
    return json.loads(urllib.request.urlopen(req, timeout=timeout).read().decode())


def catfk(*args):
    r = subprocess.run([CATFK_CLI, *args, "--json"], capture_output=True, text=True, timeout=60)
    try:
        return json.loads(r.stdout)
    except json.JSONDecodeError:
        return {"_raw": r.stdout, "_err": r.stderr}


# Merchant payment channels (CatFK shopApi/Shop/getUserChannel, token=YBNBTPYY).
# Channel ids are merchant-specific; resolved dynamically with a static fallback.
CHANNEL_CODE_BY_PAY = {"alipay": "AlipayPc", "wechat": "WeixinNative"}
CHANNEL_ID_FALLBACK = {"alipay": 1, "wechat": 4}
_channel_cache = {"at": 0, "map": {}}
MERCHANT_TOKEN = "YBNBTPYY"


def pay_channel_id(pay):
    """Map 'alipay'/'wechat' to the merchant's CatFK channel id, refreshed hourly."""
    if pay not in CHANNEL_CODE_BY_PAY:
        raise ValueError(f"unknown pay method {pay!r} (want alipay|wechat)")
    if time.time() - _channel_cache["at"] > 3600 or not _channel_cache["map"]:
        try:
            req = urllib.request.Request(
                "https://catfk.com/shopApi/Shop/getUserChannel",
                data=f"token={MERCHANT_TOKEN}".encode(),
                headers={"Content-Type": "application/x-www-form-urlencoded"},
                method="POST")
            resp = json.loads(urllib.request.urlopen(req, timeout=15).read().decode())
            channels = {}
            for ch in resp.get("data") or []:
                if ch.get("status") == 1:
                    channels[ch.get("code")] = ch.get("id")
            if channels:
                _channel_cache.update({"at": time.time(), "map": channels})
        except Exception as e:
            print(f"[checkout] channel lookup failed, using fallback: {e}", flush=True)
    code = CHANNEL_CODE_BY_PAY[pay]
    cid = _channel_cache["map"].get(code) or CHANNEL_ID_FALLBACK[pay]
    return int(cid)


def newapi_user_from_jwt(jwt):
    r = http_json(f"{NEW_API_BASE}/api/user/self", bearer=jwt)
    if not r.get("success"):
        raise ValueError("invalid jwt")
    return r["data"]["id"], (r["data"].get("email") or r["data"].get("username") or "")


def extract_cards(payload):
    """Pull card secrets from a paid Pay/query payload. Handles common shapes;
    logs the raw payload on first encounter so we can calibrate."""
    cards = []
    def walk(o):
        if isinstance(o, dict):
            if "secret" in o and isinstance(o["secret"], str):
                cards.append(o["secret"])
            else:
                for v in o.values():
                    walk(v)
        elif isinstance(o, list):
            for v in o:
                walk(v)
    walk(payload)
    return cards


def grant(order):
    kind, value = order["kind"], order["value"]
    uid = order["user_id"]
    if kind == "plan":
        r = http_json(f"{NEW_API_BASE}/api/subscription/admin/bind",
                      {"user_id": uid, "plan_id": value}, bearer=admin_token())
    else:
        r = http_json(f"{NEW_API_BASE}/api/user/manage",
                      {"id": uid, "action": "add_quota", "mode": "add", "value": value},
                      bearer=admin_token())
    return r.get("success"), r


def mark_codes_used(codes, user_id):
    """Invalidate delivered codes in new-api so the buyer cannot redeem them a
    second time after our auto-grant. Marks them exactly like a real redeem."""
    if not codes:
        return
    now = int(time.time())
    for code in codes:
        subprocess.run(["docker", "exec", "new-api-dev-pg", "psql", "-U", "root",
                        "-d", "new-api", "-c",
                        f"UPDATE redemptions SET status=3, used_user_id={user_id}, "
                        f"redeemed_time={now} WHERE key='{code}' AND status=1;"],
                       capture_output=True)


def goods_id_map():
    r = catfk("goods", "list")
    items = r.get("data") or r.get("goods") or []
    return {g.get("goods_key"): g.get("id") for g in items}


def sold_cards(goods_key, exclude_secrets=None):
    """Cards sold (delivered) for a goods. Card rows carry no trade_no or sold
    timestamp, so we return all sold cards minus ones already attributed to
    other orders."""
    gid = goods_id_map().get(goods_key)
    if not gid:
        return []
    r = catfk("stock", "list", str(gid))
    items = r.get("list") or r.get("data") or []
    excl = set(exclude_secrets or [])
    return [c for c in items if c.get("status") == 1 and c.get("secret") not in excl]


def check_and_grant(trade_no):
    orders = load_orders()
    order = orders.get(trade_no)
    if not order:
        return {"status": "error", "message": "unknown trade_no"}
    if order.get("granted"):
        return {"status": "granted", "kind": order["kind"]}
    resp = catfk("order", "status", trade_no)
    if resp.get("code") != 1:
        return {"status": "pending", "payurl": order["payurl"]}
    # paid — find the delivered cards (sold by catfk from this goods' stock,
    # minus cards already attributed to other orders)
    known = {s for o in orders.values() for s in o.get("cards", [])}
    cards = [c["secret"] for c in sold_cards(order["goods_key"], known)]
    print(f"[checkout] PAID {trade_no}, delivered cards: {cards}", flush=True)
    ok, grant_resp = grant(order)
    if ok:
        mark_codes_used(cards, order["user_id"])
        order["granted"] = True
        order["cards"] = cards
        orders[trade_no] = order
        save_orders(orders)
        return {"status": "granted", "kind": order["kind"], "cards": cards}
    return {"status": "error", "message": f"grant failed: {grant_resp}"}


def sweeper():
    while True:
        try:
            for trade_no, order in list(load_orders().items()):
                if not order.get("granted"):
                    r = check_and_grant(trade_no)
                    if r["status"] != "pending":
                        print(f"[sweeper] {trade_no}: {r['status']}", flush=True)
        except Exception as e:
            print(f"[sweeper] error: {e}", flush=True)
        time.sleep(30)


class Handler(BaseHTTPRequestHandler):
    def _json(self, code, obj):
        data = json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_OPTIONS(self):
        self.send_response(204)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.end_headers()

    def do_POST(self):
        if self.path.rstrip("/") != "/checkout":
            return self._json(404, {"error": "not found"})
        try:
            length = int(self.headers.get("Content-Length", 0))
            req = json.loads(self.rfile.read(length) or b"{}")
            goods_key = req.get("goods_key", "")
            if goods_key not in GOODS:
                return self._json(400, {"error": f"unknown goods_key {goods_key}"})
            jwt = req.get("jwt", "")
            user_id, user_email = newapi_user_from_jwt(jwt)
            kind, value = GOODS[goods_key]
            contact = req.get("contact") or user_email
            pay = (req.get("pay") or "alipay").lower()
            channel = pay_channel_id(pay)
            r = catfk("order", "create", goods_key, "--qty", "1", "--channel", str(channel),
                      "--contact", contact, "--confirm")
            trade_no, payurl = r.get("trade_no"), r.get("payurl")
            if not trade_no or not payurl:
                return self._json(502, {"error": f"catfk order create failed: {r}"})
            orders = load_orders()
            orders[trade_no] = {"trade_no": trade_no, "payurl": payurl, "user_id": user_id,
                                "goods_key": goods_key, "kind": kind, "value": value,
                                "pay": pay, "granted": False, "created_at": int(time.time())}
            save_orders(orders)
            print(f"[checkout] created {trade_no} goods={goods_key} user={user_id} pay={pay}(ch{channel})", flush=True)
            self._json(200, {"trade_no": trade_no, "payurl": payurl})
        except ValueError as e:
            self._json(401, {"error": str(e)})
        except Exception as e:
            self._json(500, {"error": str(e)})

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path.rstrip("/") == "/checkout/status":
            trade_no = parse_qs(parsed.query).get("trade_no", [""])[0]
            try:
                self._json(200, check_and_grant(trade_no))
            except Exception as e:
                self._json(500, {"error": str(e)})
        elif parsed.path.rstrip("/") == "/health":
            self._json(200, {"status": "ok"})
        else:
            self._json(404, {"error": "not found"})

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    threading.Thread(target=sweeper, daemon=True).start()
    print(f"catfk-checkout listening on :{PORT}", flush=True)
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
