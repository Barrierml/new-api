#!/usr/bin/env python3
"""migrate-par-to-newapi.py — PAR (+sub2api wallet) → local new-api data migration.

Reads production PAR (172.17.66.22:5432/par) and sub2api (172.17.66.22:5555/sub2api)
READ-ONLY, writes to the local new-api-dev Postgres via `docker exec -i new-api-dev-pg psql`.

Usage:
  uv run --with 'psycopg[binary]' --with bcrypt migrate-par-to-newapi.py [--dry-run]
      [--phase users|tokens|subscriptions|pricing|redemptions|all]

Idempotent: users anchored by remark='par:<numeric_id>', tokens/redemptions by key
(ON CONFLICT DO NOTHING), subscriptions by (user_id, plan_id, active) NOT EXISTS,
pricing by read-merge-write of option JSON. Safe to re-run; run again right before
production cutover to catch data drift.

Credentials are read from /Users/bytedance/ccgo/packages/par/.env (PAR_PG_*/PAR_SAPI_*).
"""
import argparse
import csv
import json
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from decimal import Decimal, ROUND_HALF_UP
from zoneinfo import ZoneInfo

import psycopg

PAR_ENV = "/Users/bytedance/ccgo/packages/par/.env"
LOCAL_PSQL = ["docker", "exec", "-i", "new-api-dev-pg", "psql", "-U", "root", "-d", "new-api"]
LOCAL_PSQL_RO = ["docker", "exec", "new-api-dev-pg", "psql", "-U", "root", "-d", "new-api", "-At"]
QUOTA_PER_USD = 500_000
QUOTA_INT32_MAX = 2_147_483_647
SHANGHAI = ZoneInfo("Asia/Shanghai")
# PAR expires_at NULL means unlimited; new-api needs a concrete end_time.
FAR_FUTURE_END = 4102444800  # 2100-01-01 UTC

# PAR plan name -> new-api plan title (identical names, verified 2026-07-21)
SUBSCRIPTION_PLANS = ["Mini", "Pro mini", "Pro x1", "Pro x2", "Pro x3", "Pro x4"]


# ---------------------------------------------------------------- helpers

def load_env(path):
    env = {}
    for line in open(path):
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            env[k.strip()] = v.strip().strip('"').strip("'")
    return env


def prod_conn(env, prefix):
    return psycopg.connect(
        host=env[f"{prefix}_HOST"], port=env.get(f"{prefix}_PORT", "5432"),
        user=env[f"{prefix}_USER"], password=env[f"{prefix}_PASS"],
        dbname=env.get(f"{prefix}_DB") or env.get(f"{prefix}_NAME"),
        options="-c default_transaction_read_only=on -c statement_timeout=120000",
        gssencmode="disable")


def lit(v):
    """SQL literal for the local psql pipe."""
    if v is None:
        return "NULL"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float, Decimal)):
        return str(v)
    return "'" + str(v).replace("'", "''") + "'"


def local_sql(sql):
    """Execute write SQL against local new-api PG. Raises on error."""
    r = subprocess.run(LOCAL_PSQL + ["-v", "ON_ERROR_STOP=1", "--single-transaction"],
                       input=sql.encode(), capture_output=True)
    if r.returncode != 0:
        raise RuntimeError(f"local psql failed:\n{r.stderr.decode()[:2000]}")
    return r.stdout.decode()


def local_query(sql):
    r = subprocess.run(LOCAL_PSQL_RO + ["-c", sql], capture_output=True)
    if r.returncode != 0:
        raise RuntimeError(f"local query failed:\n{r.stderr.decode()[:1000]}")
    return [l for l in r.stdout.decode().splitlines() if l.strip()]


def monday_anchors(now=None):
    """(this Monday 00:00, next Monday 00:00) Asia/Shanghai, as epoch ints."""
    now = now or datetime.now(SHANGHAI)
    monday = (now - timedelta(days=now.weekday())).replace(hour=0, minute=0, second=0, microsecond=0)
    return int(monday.timestamp()), int((monday + timedelta(days=7)).timestamp())


# ---------------------------------------------------------------- extract

def extract(env):
    data = {}
    with prod_conn(env, "PAR_PG") as cx, cx.cursor() as cur:
        cur.execute("""SELECT id, numeric_id, name, email, is_admin, password_hash,
                              EXTRACT(EPOCH FROM created_at)::bigint
                       FROM par_users""")
        data["users"] = cur.fetchall()
        cur.execute("""SELECT k.user_id, k.par_key, k.sapi_user_id, k.is_primary,
                              EXTRACT(EPOCH FROM k.created_at)::bigint
                       FROM par_keys k WHERE k.status='active'""")
        data["keys"] = cur.fetchall()
        cur.execute("""SELECT u.numeric_id, p.name,
                              EXTRACT(EPOCH FROM s.starts_at)::bigint,
                              EXTRACT(EPOCH FROM s.expires_at)::bigint
                       FROM par_subscriptions s
                       JOIN par_plans p ON p.id = s.plan_id
                       JOIN par_users u ON u.id = s.user_id
                       WHERE s.status='active' AND p.type='subscription'
                         AND (s.expires_at IS NULL OR s.expires_at > now())""")
        data["subscriptions"] = cur.fetchall()
        cur.execute("""SELECT code, type, topup_amount,
                              EXTRACT(EPOCH FROM expires_at)::bigint,
                              EXTRACT(EPOCH FROM created_at)::bigint
                       FROM par_redeem_codes WHERE uses_count < max_uses""")
        data["codes"] = cur.fetchall()
        cur.execute("""SELECT id, input_price, output_price, cache_creation_price,
                              cache_read_price, COALESCE(multiplier, 1)
                       FROM par_models WHERE visible""")
        data["models"] = cur.fetchall()
    with prod_conn(env, "PAR_SAPI_DB") as cx, cx.cursor() as cur:
        cur.execute("SELECT id, balance FROM users")
        data["sapi_balance"] = {r[0]: Decimal(str(r[1])) for r in cur.fetchall()}
    return data


# ---------------------------------------------------------------- phases

def phase_users(data, dry):
    # map PAR numeric_id -> plan name (active subscription) and sapi balance
    plan_by_user = {nid: pname for nid, pname, _, _ in data["subscriptions"]}
    sapi_by_user = {}
    for uid_uuid, par_key, sapi_uid, _prim, _ts in data["keys"]:
        sapi_by_user.setdefault(uid_uuid, sapi_uid)

    existing_users = {}
    for row in local_query('SELECT id, username, COALESCE(remark,\'\') FROM users'):
        parts = row.split("|", 2)
        existing_users.setdefault(parts[1], parts[0])
    migrated = {r.split("|")[2].split()[0] for r in local_query(
        "SELECT id, username, remark FROM users WHERE remark LIKE 'par:%'") if len(r.split("|")) >= 3}
    migrated_nids = {int(m.split(":")[1]) for m in migrated if m.startswith("par:")}

    plan_group = {"Mini": "mini", "Pro mini": "pro-mini", "Pro x1": "pro-x1",
                  "Pro x2": "pro-x2", "Pro x3": "pro-x3", "Pro x4": "pro-x4"}

    rows, report = [], {"bcrypt": 0, "reset_flag": [], "no_sapi": [], "negative": [],
                        "clamped": [], "username_fallback": [], "skipped_existing": 0}
    import bcrypt as _bc, secrets as _sec
    used_usernames = set(existing_users.keys())
    for uuid, nid, name, email, is_admin, pw_hash, created_ts in data["users"]:
        if nid in migrated_nids:
            report["skipped_existing"] += 1
            continue
        email = (email or "").strip()
        if email and len(email) <= 20 and email.lower() not in {u.lower() for u in used_usernames}:
            username = email
        else:
            username = f"u{nid}"
            report["username_fallback"].append((nid, email))
        used_usernames.add(username)
        if pw_hash and pw_hash.startswith("$2"):
            password = pw_hash
            report["bcrypt"] += 1
        else:
            password = _bc.hashpw(_sec.token_urlsafe(24).encode(), _bc.gensalt()).decode()
            report["reset_flag"].append((nid, email, "argon2" if pw_hash else "oauth"))
        sapi_uid = sapi_by_user.get(uuid)
        balance = data["sapi_balance"].get(sapi_uid, Decimal(0)) if sapi_uid else Decimal(0)
        if not sapi_uid:
            report["no_sapi"].append((nid, email))
        if balance < 0:
            report["negative"].append((nid, email, str(balance)))
        quota = int((balance * QUOTA_PER_USD).quantize(Decimal("1"), rounding=ROUND_HALF_UP))
        quota = max(0, quota)
        if quota > QUOTA_INT32_MAX:
            report["clamped"].append((nid, email, quota))
            quota = QUOTA_INT32_MAX
        group = plan_group.get(plan_by_user.get(nid, ""), "default")
        role = 10 if is_admin else 1
        remark = f"par:{nid} uuid:{uuid}"
        rows.append(f"({lit(username)},{lit(password)},{lit((name or '')[:20])},{role},1,"
                    f"{lit(email)},{quota},0,{lit(group)},{lit('m'+str(nid))},{lit(remark)},{created_ts})")

    sql = ""
    for i in range(0, len(rows), 50):
        batch = ",\n".join(rows[i:i+50])
        sql += ("INSERT INTO users (username,password,display_name,role,status,email,quota,"
                "used_quota,\"group\",aff_code,remark,created_at) VALUES\n" + batch +
                "\nON CONFLICT (username) DO NOTHING;\n")
    if not dry and sql:
        local_sql(sql)
    report["inserted"] = len(rows)
    return report


def build_uid_map():
    uid_map = {}
    for row in local_query("SELECT id, remark FROM users WHERE remark LIKE 'par:%'"):
        uid, remark = row.split("|", 1)
        nid = remark.split()[0].split(":")[1]
        uid_map[int(nid)] = int(uid)
    return uid_map


def phase_tokens(data, uid_map, dry):
    uuid_to_nid = {u[0]: u[1] for u in data["users"]}
    rows = []
    for uid_uuid, par_key, _sapi, _prim, created_ts in data["keys"]:
        new_uid = uid_map.get(uuid_to_nid.get(uid_uuid))
        if not new_uid:
            continue
        rows.append(f"({new_uid},{lit(par_key)},1,'par-migrated',{created_ts},{created_ts},"
                    f"-1,0,true,false,'','',0,'')")
    sql = ""
    for i in range(0, len(rows), 50):
        batch = ",\n".join(rows[i:i+50])
        sql += ("INSERT INTO tokens (user_id,key,status,name,created_time,accessed_time,"
                "expired_time,remain_quota,unlimited_quota,model_limits_enabled,model_limits,"
                "allow_ips,used_quota,\"group\") VALUES\n" + batch +
                "\nON CONFLICT (key) DO NOTHING;\n")
    if not dry and sql:
        local_sql(sql)
    return {"inserted": len(rows)}


def phase_subscriptions(data, uid_map, dry):
    plans = {}
    for row in local_query("SELECT id, title, upgrade_group, downgrade_group, total_amount FROM subscription_plans"):
        pid, title, ug, dg, total = row.split("|")
        plans[title] = {"id": int(pid), "ug": ug, "dg": dg, "total": int(total)}
    last_reset, next_reset = monday_anchors()
    now_ts = int(datetime.now(timezone.utc).timestamp())
    rows, report = [], {"null_expiry": []}
    for nid, pname, starts_at, expires_at in data["subscriptions"]:
        new_uid = uid_map.get(nid)
        plan = plans.get(pname)
        if not new_uid or not plan:
            continue
        end = expires_at if expires_at else FAR_FUTURE_END
        if not expires_at:
            report["null_expiry"].append((nid, pname))
        rows.append(
            f"SELECT {new_uid},{plan['id']},{plan['total']},0,{starts_at},{end},"
            f"'active','migration',{last_reset},{next_reset},{lit(plan['ug'])},'default',"
            f"{lit(plan['dg'])},false,{now_ts},{now_ts}\nWHERE NOT EXISTS ("
            f"SELECT 1 FROM user_subscriptions WHERE user_id={new_uid} "
            f"AND plan_id={plan['id']} AND status='active')")
    sql = ""
    for r in rows:
        sql += ("INSERT INTO user_subscriptions (user_id,plan_id,amount_total,amount_used,"
                "start_time,end_time,status,source,last_reset_time,next_reset_time,"
                "upgrade_group,prev_user_group,downgrade_group,allow_wallet_overflow,"
                "created_at,updated_at)\n" + r + ";\n")
    if not dry and sql:
        local_sql(sql)
    report["inserted"] = len(rows)
    report["next_reset"] = next_reset
    return report


def phase_pricing(data, dry):
    ratio_maps = {"ModelRatio": {}, "CompletionRatio": {}, "CacheRatio": {}, "CreateCacheRatio": {}}
    for key in ratio_maps:
        rows = local_query(f"SELECT value FROM options WHERE key='{key}'")
        if rows:
            ratio_maps[key] = json.loads(rows[0])
    report = {"models": 0, "skipped": [], "anchors": {}}
    for mid, pin, pout, pcc, pcr, mult in data["models"]:
        pin, pout = Decimal(str(pin)), Decimal(str(pout))
        mult = Decimal(str(mult))
        if not pin or not pout:
            report["skipped"].append(mid)
            continue
        eff_in = pin * mult
        eff_out = pout * mult
        ratio_maps["ModelRatio"][mid] = float(eff_in / 2)
        ratio_maps["CompletionRatio"][mid] = float(eff_out / eff_in)
        if pcr and Decimal(str(pcr)) > 0:
            ratio_maps["CacheRatio"][mid] = float(Decimal(str(pcr)) / pin)
        if pcc and Decimal(str(pcc)) > 0:
            ratio_maps["CreateCacheRatio"][mid] = float(Decimal(str(pcc)) / pin)
        report["models"] += 1
    # sanity anchors
    assert abs(ratio_maps["ModelRatio"]["claude-fable-5"] - 55.0) < 0.01, "fable-5 anchor off"
    assert abs(ratio_maps["CompletionRatio"]["gpt-5.4"] - 6.0) < 0.01, "gpt-5.4 anchor off"
    report["anchors"] = {"claude-fable-5": ratio_maps["ModelRatio"]["claude-fable-5"],
                         "gpt-5.4": ratio_maps["ModelRatio"]["gpt-5.4"],
                         "deepseek-v3.2": ratio_maps["ModelRatio"]["deepseek-v3.2"]}
    sql = ""
    for key, m in ratio_maps.items():
        sql += (f"INSERT INTO options (key, value) VALUES ({lit(key)}, {lit(json.dumps(m))}) "
                f"ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;\n")
    if not dry:
        local_sql(sql)
    return report


def phase_redemptions(data, dry):
    rows, plan_codes = [], []
    now_ts = int(datetime.now(timezone.utc).timestamp())
    for code, ctype, amount, expires_ts, created_ts in data["codes"]:
        if ctype == "topup":
            quota = int((Decimal(str(amount)) * QUOTA_PER_USD).quantize(
                Decimal("1"), rounding=ROUND_HALF_UP))
            rows.append(f"({lit(code)},'par-migration',{quota},1,{created_ts},{expires_ts or 0},1,0)")
        else:
            plan_codes.append({"code": code, "expires_at": expires_ts, "created_at": created_ts})
    sql = ""
    for i in range(0, len(rows), 50):
        batch = ",\n".join(rows[i:i+50])
        sql += ("INSERT INTO redemptions (key,name,quota,status,created_time,expired_time,"
                "user_id,used_user_id) VALUES\n" + batch + "\nON CONFLICT (key) DO NOTHING;\n")
    if not dry and sql:
        local_sql(sql)
    return {"topup_inserted": len(rows), "plan_codes": plan_codes}


# ---------------------------------------------------------------- main

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--phase", default="all",
                    choices=["users", "tokens", "subscriptions", "pricing", "redemptions", "all"])
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    env = load_env(PAR_ENV)
    print("extracting from prod (read-only)...", flush=True)
    data = extract(env)
    print(f"  users={len(data['users'])} keys={len(data['keys'])} "
          f"subs={len(data['subscriptions'])} codes={len(data['codes'])} "
          f"models={len(data['models'])} sapi_balances={len(data['sapi_balance'])}")

    full = {}
    phases = [args.phase] if args.phase != "all" else \
        ["users", "tokens", "subscriptions", "pricing", "redemptions"]
    uid_map = None
    for ph in phases:
        if ph == "users":
            full["users"] = phase_users(data, args.dry_run)
            print(f"users: {json.dumps(full['users'], default=str)[:400]}")
        if not args.dry_run and uid_map is None and ph in ("tokens", "subscriptions"):
            uid_map = build_uid_map()
        elif ph in ("tokens", "subscriptions") and args.dry_run:
            uid_map = uid_map or {}
        if ph == "tokens":
            full["tokens"] = phase_tokens(data, uid_map or build_uid_map(), args.dry_run)
            print(f"tokens: {full['tokens']}")
        if ph == "subscriptions":
            full["subscriptions"] = phase_subscriptions(data, uid_map or build_uid_map(), args.dry_run)
            print(f"subscriptions: {json.dumps(full['subscriptions'], default=str)[:300]}")
        if ph == "pricing":
            full["pricing"] = phase_pricing(data, args.dry_run)
            print(f"pricing: {full['pricing']}")
        if ph == "redemptions":
            full["redemptions"] = phase_redemptions(data, args.dry_run)
            print(f"redemptions: topup={full['redemptions']['topup_inserted']} "
                  f"plan_codes={len(full['redemptions']['plan_codes'])}")

    # reports
    with open("par_plan_codes.csv", "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["code", "expires_at", "created_at"])
        for c in full.get("redemptions", {}).get("plan_codes", []):
            w.writerow([c["code"], c["expires_at"], c["created_at"]])
    with open("migration_report.json", "w") as f:
        json.dump(full, f, default=str, ensure_ascii=False, indent=1)
    print("wrote migration_report.json + par_plan_codes.csv")
    if args.dry_run:
        print("DRY RUN — nothing written to DB")


if __name__ == "__main__":
    main()
