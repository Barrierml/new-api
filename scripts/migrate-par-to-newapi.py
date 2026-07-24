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
import os
import shlex
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from decimal import Decimal, ROUND_HALF_UP
from zoneinfo import ZoneInfo

import psycopg

PAR_ENV = "/Users/bytedance/ccgo/packages/par/.env"
# Target DB psql command. Default = local new-api-dev container.
# Override via $TAKO_PSQL for a remote target, e.g.
#   TAKO_PSQL="psql -h 172.17.66.24 -p 15432 -U tako -d tako" (set PGPASSWORD too)
LOCAL_PSQL = shlex.split(
    os.environ.get("TAKO_PSQL", "docker exec -i new-api-dev-pg psql -U root -d new-api")
)
LOCAL_PSQL_RO = LOCAL_PSQL + ["-At"]
QUOTA_PER_USD = 500_000
BALANCE_FACTOR = Decimal("3") / Decimal("2")  # PAR wallet -> Tako wallet multiplier
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
                       FROM par_users
                       WHERE COALESCE(is_admin, false) = false""")
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
        cur.execute("""SELECT c.code, c.type, c.topup_amount,
                              EXTRACT(EPOCH FROM c.expires_at)::bigint,
                              EXTRACT(EPOCH FROM c.created_at)::bigint,
                              p.name
                       FROM par_redeem_codes c LEFT JOIN par_plans p ON p.id = c.plan_id
                       WHERE c.uses_count < c.max_uses""")
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
    """Insert missing non-admin users and refresh wallet/password/group.

    Rules for tonight's cutover:
    - skip admins (already filtered in extract)
    - wallet quota = max(0, round(sapi_balance * 1.5 * 500000)) and OVERWRITE
    - bcrypt password hashes are synced so users can log in with PAR password
    - argon2 / empty passwords are not force-overwritten for existing users
    - existing users are updated in place; missing users are inserted
    """
    plan_by_user = {nid: pname for nid, pname, _, _ in data["subscriptions"]}
    sapi_by_user = {}
    for uid_uuid, par_key, sapi_uid, _prim, _ts in data["keys"]:
        if uid_uuid not in sapi_by_user and sapi_uid:
            sapi_by_user[uid_uuid] = sapi_uid

    # existing tako users: username -> id, and par_nid -> row info
    existing_by_username = {}
    existing_by_nid = {}
    for row in local_query(
        "SELECT id, username, COALESCE(remark,''), COALESCE(password,''), quota, \"group\" FROM users"
    ):
        parts = row.split("|", 5)
        if len(parts) < 6:
            continue
        uid, username, remark, password, quota_s, group = parts
        existing_by_username[username] = uid
        if remark.startswith("par:"):
            try:
                nid = int(remark.split()[0].split(":", 1)[1])
            except Exception:
                continue
            existing_by_nid[nid] = {
                "id": int(uid),
                "username": username,
                "password": password,
                "quota": int(quota_s or 0),
                "group": group,
                "remark": remark,
            }

    plan_group = {
        "Mini": "mini",
        "Pro mini": "pro-mini",
        "Pro x1": "pro-x1",
        "Pro x2": "pro-x2",
        "Pro x3": "pro-x3",
        "Pro x4": "pro-x4",
    }

    import bcrypt as _bc
    import secrets as _sec

    used_usernames = set(existing_by_username.keys())
    inserts = []
    updates = []
    report = {
        "skipped_admin": 0,
        "insert_candidates": 0,
        "update_candidates": 0,
        "bcrypt_sync": 0,
        "argon2_reset_needed": [],
        "empty_password": [],
        "no_sapi": [],
        "negative": [],
        "clamped": [],
        "username_fallback": [],
        "balance_changes": [],
        "sum_old_quota": 0,
        "sum_new_quota": 0,
        "inserted": 0,
        "updated": 0,
    }

    for uuid, nid, name, email, is_admin, pw_hash, created_ts in data["users"]:
        if is_admin:
            report["skipped_admin"] += 1
            continue

        email = (email or "").strip()
        existing = existing_by_nid.get(nid)

        # username selection only for inserts
        if existing is None:
            if email and len(email) <= 20 and email.lower() not in {u.lower() for u in used_usernames}:
                username = email
            else:
                username = f"u{nid}"
                report["username_fallback"].append((nid, email))
            # avoid collisions with already planned inserts
            base = username
            i = 2
            while username in used_usernames:
                username = f"{base}_{i}"
                i += 1
            used_usernames.add(username)
        else:
            username = existing["username"]

        # password handling
        pw_kind = "empty"
        password = None
        if pw_hash and pw_hash.startswith("$2"):
            pw_kind = "bcrypt"
            password = pw_hash
            report["bcrypt_sync"] += 1
        elif pw_hash and pw_hash.startswith("$argon2"):
            pw_kind = "argon2"
            report["argon2_reset_needed"].append((nid, email, name))
            if existing is None:
                password = _bc.hashpw(_sec.token_urlsafe(24).encode(), _bc.gensalt()).decode()
        else:
            report["empty_password"].append((nid, email, name))
            if existing is None:
                password = _bc.hashpw(_sec.token_urlsafe(24).encode(), _bc.gensalt()).decode()

        # wallet balance * 1.5
        sapi_uid = sapi_by_user.get(uuid)
        balance = data["sapi_balance"].get(sapi_uid, Decimal(0)) if sapi_uid else Decimal(0)
        if not sapi_uid:
            report["no_sapi"].append((nid, email))
        if balance < 0:
            report["negative"].append((nid, email, str(balance)))
            balance = Decimal(0)
        quota = int((balance * BALANCE_FACTOR * QUOTA_PER_USD).quantize(Decimal("1"), rounding=ROUND_HALF_UP))
        quota = max(0, quota)
        if quota > QUOTA_INT32_MAX:
            report["clamped"].append((nid, email, quota))
            quota = QUOTA_INT32_MAX

        group = plan_group.get(plan_by_user.get(nid, ""), "default")
        remark = f"par:{nid} uuid:{uuid}"
        if pw_kind == "argon2":
            remark += " pw_reset:argon2"
        elif pw_kind == "empty":
            remark += " pw_reset:empty"

        if existing is None:
            report["insert_candidates"] += 1
            inserts.append(
                f"({lit(username)},{lit(password)},{lit((name or '')[:20])},1,1,"
                f"{lit(email)},{quota},0,{lit(group)},{lit('m'+str(nid))},{lit(remark)},{created_ts})"
            )
            report["balance_changes"].append({
                "nid": nid, "email": email, "action": "insert",
                "old_quota": 0, "new_quota": quota, "balance": str(balance),
            })
            report["sum_new_quota"] += quota
        else:
            report["update_candidates"] += 1
            old_quota = existing["quota"]
            report["sum_old_quota"] += old_quota
            report["sum_new_quota"] += quota
            report["balance_changes"].append({
                "nid": nid, "email": email, "action": "update",
                "old_quota": old_quota, "new_quota": quota, "balance": str(balance),
                "delta": quota - old_quota,
            })
            set_parts = [
                f"quota={quota}",
                f"\"group\"={lit(group)}",
                f"remark={lit(remark)}",
            ]
            # only overwrite password when PAR has a usable bcrypt hash
            if password is not None and pw_kind == "bcrypt":
                set_parts.append(f"password={lit(password)}")
            if email:
                set_parts.append(f"email={lit(email)}")
            if name:
                set_parts.append(f"display_name={lit((name or '')[:20])}")
            updates.append(f"UPDATE users SET {', '.join(set_parts)} WHERE id={existing['id']};")

    sql = ""
    for i in range(0, len(inserts), 50):
        batch = ",\n".join(inserts[i:i+50])
        sql += (
            "INSERT INTO users (username,password,display_name,role,status,email,quota,"
            "used_quota,\"group\",aff_code,remark,created_at) VALUES\n" + batch +
            "\nON CONFLICT (username) DO NOTHING;\n"
        )
    if updates:
        sql += "\n".join(updates) + "\n"

    if not dry and sql:
        local_sql(sql)
    report["inserted"] = len(inserts)
    report["updated"] = len(updates)
    # keep only top balance deltas in printed report later; store all for json
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
    for row in local_query(
        "SELECT id, title, upgrade_group, downgrade_group, total_amount, COALESCE(sub_quota_limits,'') "
        "FROM subscription_plans"
    ):
        parts = row.split("|")
        if len(parts) < 6:
            continue
        pid, title, ug, dg, total, limits = parts
        plans[title] = {
            "id": int(pid),
            "ug": ug,
            "dg": dg,
            "total": int(total),
            "limits": limits,
        }
    last_reset, next_reset = monday_anchors()
    now_ts = int(datetime.now(timezone.utc).timestamp())
    rows, report = [], {
        "null_expiry": [],
        "inserted": 0,
        "already_active": 0,
        "limits_backfill": 0,
        "missing_user": [],
        "missing_plan": [],
    }
    for nid, pname, starts_at, expires_at in data["subscriptions"]:
        new_uid = uid_map.get(nid)
        plan = plans.get(pname)
        if not new_uid:
            report["missing_user"].append((nid, pname))
            continue
        if not plan:
            report["missing_plan"].append((nid, pname))
            continue
        end = expires_at if expires_at else FAR_FUTURE_END
        if not expires_at:
            report["null_expiry"].append((nid, pname))
        # check whether active row already exists
        exists = local_query(
            f"SELECT id FROM user_subscriptions WHERE user_id={new_uid} "
            f"AND plan_id={plan['id']} AND status='active' LIMIT 1"
        )
        if exists:
            report["already_active"] += 1
            continue
        rows.append(
            f"({new_uid},{plan['id']},{plan['total']},0,{starts_at},{end},"
            f"'active','migration',{last_reset},{next_reset},{lit(plan['ug'])},'default',"
            f"{lit(plan['dg'])},false,{lit(plan['limits'])},{now_ts},{now_ts})"
        )
    sql = ""
    if rows:
        for i in range(0, len(rows), 50):
            batch = ",\n".join(rows[i:i+50])
            sql += (
                "INSERT INTO user_subscriptions (user_id,plan_id,amount_total,amount_used,"
                "start_time,end_time,status,source,last_reset_time,next_reset_time,"
                "upgrade_group,prev_user_group,downgrade_group,allow_wallet_overflow,"
                "sub_quota_limits,created_at,updated_at) VALUES\n" + batch + ";\n"
            )
    # always backfill empty plan snapshots for active subs from plan definition
    sql += (
        "UPDATE user_subscriptions us SET "
        "sub_quota_limits = sp.sub_quota_limits, updated_at = "
        f"{now_ts} "
        "FROM subscription_plans sp "
        "WHERE us.plan_id = sp.id AND us.status = 'active' "
        "AND (us.sub_quota_limits IS NULL OR us.sub_quota_limits = '' OR us.sub_quota_limits = '{}');\n"
    )
    empty_before = local_query(
        "SELECT count(*) FROM user_subscriptions WHERE status='active' "
        "AND (sub_quota_limits IS NULL OR sub_quota_limits='' OR sub_quota_limits='{}')"
    )
    report["limits_backfill"] = int(empty_before[0]) if empty_before else 0
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
    plan_ids = {}
    for row in local_query("SELECT id, title FROM subscription_plans"):
        pid, title = row.split("|")
        plan_ids[title] = int(pid)
    rows, plan_skipped = [], []
    for code, ctype, amount, expires_ts, created_ts, plan_name in data["codes"]:
        if ctype == "topup":
            quota = int((Decimal(str(amount)) * QUOTA_PER_USD).quantize(
                Decimal("1"), rounding=ROUND_HALF_UP))
            rows.append(f"({lit(code)},'par-migration',{quota},1,{created_ts},{expires_ts or 0},1,0,'quota',0)")
        else:
            pid = plan_ids.get(plan_name)
            if not pid:
                plan_skipped.append(code)
                continue
            rows.append(f"({lit(code)},'par-migration',0,1,{created_ts},{expires_ts or 0},1,0,'subscription',{pid})")
    sql = ""
    for i in range(0, len(rows), 50):
        batch = ",\n".join(rows[i:i+50])
        sql += ("INSERT INTO redemptions (key,name,quota,status,created_time,expired_time,"
                "user_id,used_user_id,redemption_type,subscription_plan_id) VALUES\n" + batch +
                "\nON CONFLICT (key) DO NOTHING;\n")
    if not dry and sql:
        local_sql(sql)
    topup_n = sum(1 for c in data["codes"] if c[1] == "topup")
    return {"topup_inserted": topup_n, "plan_inserted": len(rows) - topup_n,
            "plan_skipped": plan_skipped}


# ---------------------------------------------------------------- main

def _print_users_report(rep):
    print("users:")
    print(f"  insert={rep.get('insert_candidates')} update={rep.get('update_candidates')} "
          f"bcrypt_sync={rep.get('bcrypt_sync')}")
    print(f"  sum_old_quota={rep.get('sum_old_quota')} sum_new_quota={rep.get('sum_new_quota')} "
          f"factor={BALANCE_FACTOR}")
    argon = rep.get("argon2_reset_needed") or []
    print(f"  argon2_reset_needed={len(argon)}")
    for item in argon[:10]:
        print(f"    - nid={item[0]} email={item[1]} name={item[2]}")
    empty = rep.get("empty_password") or []
    print(f"  empty_password={len(empty)}")
    changes = rep.get("balance_changes") or []
    ranked = sorted(
        changes,
        key=lambda x: abs(int(x.get("delta", x.get("new_quota", 0) - x.get("old_quota", 0)))),
        reverse=True,
    )
    print("  top balance changes:")
    for c in ranked[:12]:
        old_q = c.get("old_quota", 0)
        new_q = c.get("new_quota", 0)
        print(f"    - nid={c['nid']} email={c.get('email')} "
              f"{old_q}->{new_q} (bal={c.get('balance')}) action={c.get('action')}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--phase", default="cutover",
                    choices=["users", "tokens", "subscriptions", "pricing", "redemptions", "all", "cutover"])
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    env = load_env(PAR_ENV)
    print("extracting from prod (read-only)...", flush=True)
    data = extract(env)
    print(f"  users={len(data['users'])} keys={len(data['keys'])} "
          f"subs={len(data['subscriptions'])} codes={len(data['codes'])} "
          f"models={len(data['models'])} sapi_balances={len(data['sapi_balance'])}")

    full = {}
    if args.phase == "cutover":
        phases = ["users", "tokens", "subscriptions"]
    elif args.phase == "all":
        phases = ["users", "tokens", "subscriptions", "pricing", "redemptions"]
    else:
        phases = [args.phase]

    uid_map = None
    for ph in phases:
        if ph == "users":
            full["users"] = phase_users(data, args.dry_run)
            _print_users_report(full["users"])
        if ph in ("tokens", "subscriptions"):
            uid_map = build_uid_map()
        if ph == "tokens":
            full["tokens"] = phase_tokens(data, uid_map, args.dry_run)
            print(f"tokens: {full['tokens']}")
        if ph == "subscriptions":
            full["subscriptions"] = phase_subscriptions(data, uid_map, args.dry_run)
            print(f"subscriptions: {json.dumps(full['subscriptions'], default=str)[:500]}")
        if ph == "pricing":
            full["pricing"] = phase_pricing(data, args.dry_run)
            print(f"pricing: {full['pricing']}")
        if ph == "redemptions":
            full["redemptions"] = phase_redemptions(data, args.dry_run)
            print(f"redemptions: topup={full['redemptions']['topup_inserted']} "
                  f"plan={full['redemptions']['plan_inserted']} "
                  f"skipped={full['redemptions']['plan_skipped']}")

    with open("migration_report.json", "w") as f:
        json.dump(full, f, default=str, ensure_ascii=False, indent=1)
    print("wrote migration_report.json")
    if args.dry_run:
        print("DRY RUN — nothing written to DB")


if __name__ == "__main__":
    main()
