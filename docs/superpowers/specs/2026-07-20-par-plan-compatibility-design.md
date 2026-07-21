# PAR-Compatible Subscription Plans

> **2026-07-21 决策更新(本文档原设计已作废,不实施)**
>
> 两项决策变化使本 spec 的 schema 扩展方案不再必要:
>
> 1. **限制维度 descope**:套餐只保留「周额度 + RPM」两层,5h 滚动窗、日额度、
>    allowed_models 均不做。
> 2. **零代码实现**:调查发现 stock new-api 已具备等价能力,无需新增任何列:
>    - 周额度 = 现有 `total_amount` + `quota_reset_period='weekly'`
>      (`calcNextResetTime` weekly 分支本就锚定下周一 00:00 server TZ,
>      `maybeResetUserSubscriptionWithPlanTx` 惰性重置 + 后台 task 双保险)
>    - RPM = `upgrade_group` 分组升级 + `ModelRequestRateLimitGroup` 分组限流
>      (middleware/model-rate-limit.go,按 userId 计数,一人一档下等价于每订阅)
>    - 耗尽硬停 = `allow_wallet_overflow=false`
>    - CNY 价格:controller 强制 `currency=USD` 且前端价格硬编码 `$`,
>      零代码方案下作为已知展示限制接受(subtitle 标注 ¥ 价格对冲);
>      若未来要修,最小补丁 = 去掉强制 USD + 前端 `formatPlanPrice` helper(约 30 行)
>
> 2026-07-21 已在本地 dev 实例(docker-compose.dev.yml)按此方案完成配置并通过
> E2E 验证(周记账 / RPM 429 / 周一重置 / 耗尽 403 硬停 / 到期降级),详见
> my-documents `changelog/2026-07-21.md`。

## Goal

Extend New API subscription plans so the current PAR paid plans can be migrated
without flattening their rolling-window, daily, weekly, model-access, and request
rate rules into a single quota bucket. Plan prices must retain their original CNY
currency while usage limits continue to use New API quota units derived from USD.

The first migration target is the live PAR catalog on 2026-07-20:

| Plan | Price CNY | 5h quota USD | Daily quota USD | Weekly quota USD | RPM |
|---|---:|---:|---:|---:|---:|
| Mini | 59 | 0 | 30 | 74 | 20 |
| Pro mini | 119 | 14 | 40 | 140 | 10 |
| Pro x1 | 199 | 24 | 64 | 240 | 20 |
| Pro x2 | 329 | 38 | 120 | 400 | 20 |
| Pro x3 | 499 | 61 | 185 | 610 | 30 |
| Pro x4 | 749 | 90 | 285 | 930 | 40 |

All paid plans are sold as 30-day subscriptions. Payment-provider integration
and production PAR user migration are outside this first implementation slice.

## Non-Goals

- Do not implement CatFK payment yet.
- Do not migrate PAR users, keys, subscriptions, redemption codes, or balances.
- Do not replace New API wallet billing or the existing `total_amount` quota.
- Do not add provider-specific or database-specific SQL.
- Do not change existing subscriptions when an administrator edits a plan.

## Plan Configuration

Add these fields to `SubscriptionPlan`:

| Field | Type | Meaning |
|---|---|---|
| `window_amount` | `int64` | Quota units allowed in the rolling window; `0` disables the limit |
| `window_minutes` | `int` | Rolling-window duration; required when `window_amount > 0` |
| `daily_amount` | `int64` | Quota units allowed per local calendar day; `0` disables the limit |
| `weekly_amount` | `int64` | Quota units allowed per Monday-based local week; `0` disables the limit |
| `rpm_limit` | `int` | Maximum requests per minute; `0` disables the limit |
| `allowed_models` | text JSON | Optional array of exact model IDs; empty means unrestricted |

Keep `total_amount` and `quota_reset_period` unchanged for native New API plans.
The additional limits are independent guards. A request is allowed only when it
fits every enabled quota window.

Allow `currency` values `USD` and `CNY`. Remove the controller behavior that
unconditionally overwrites the submitted currency with `USD`. Currency changes
display and payment amount interpretation only; quota limits remain quota units.

Use text-backed JSON for `allowed_models`, parsed through `common.Marshal` and
`common.Unmarshal`, so SQLite, MySQL, and PostgreSQL share the same behavior.

## Subscription Snapshots

New API already snapshots `total_amount`, group transitions, and wallet overflow
onto `UserSubscription`. Apply the same rule to the new fields so editing a plan
does not retroactively alter an already purchased subscription.

Add these snapshot and accounting fields to `UserSubscription`:

| Field | Purpose |
|---|---|
| `window_amount` / `window_amount_used` | Rolling-window limit and usage |
| `window_minutes` / `window_started_at` | Rolling-window duration and anchor |
| `daily_amount` / `daily_amount_used` | Daily limit and usage |
| `daily_started_at` | Current local-day anchor |
| `weekly_amount` / `weekly_amount_used` | Weekly limit and usage |
| `weekly_started_at` | Current Monday-based week anchor |
| `rpm_limit` | Per-subscription request rate snapshot |
| `allowed_models` | Model allowlist snapshot |

Existing rows receive zero-value fields and remain behaviorally unchanged.

## Billing Enforcement

Extend `PreConsumeUserSubscription` rather than creating a second billing path.
Inside the existing transaction and row lock:

1. Load active subscriptions in the existing priority order.
2. Refresh expired rolling, daily, and weekly counters in memory.
3. Reject a subscription when the requested model is not in its allowlist.
4. Reject a subscription when pre-consumption exceeds any enabled limit.
5. Increment total, rolling, daily, and weekly counters atomically.
6. Record every changed counter in the pre-consume record.

`RefundSubscriptionPreConsume` and `PostConsumeUserSubscriptionDelta` must apply
the same delta to every counter charged by the original request. Counters are
clamped at zero during refunds.

RPM enforcement should reuse the existing Redis/in-memory rate-limit layer with
a key scoped to the active user subscription. Database counters are not suitable
for per-request minute windows.

## Reset Semantics

- Rolling window: anchored when the first charged request enters an empty or
  expired window; reset after `window_minutes`.
- Daily: reset at the next local midnight using the server timezone.
- Weekly: reset at Monday 00:00 using the server timezone.
- Existing `quota_reset_period`: continues to reset `amount_used` only.
- Subscription expiry: remains managed by the existing maintenance task.

Lazy reset during pre-consumption is authoritative. The background task may later
reset due counters for cleaner UI, but request correctness must not depend on the
task running on time.

## Admin And User UI

Extend the subscription plan form with a dedicated "Usage limits" section:

- Price currency (`USD` or `CNY`)
- Rolling-window quota and minutes
- Daily quota
- Weekly quota
- Requests per minute
- Model allowlist using the existing model selector patterns

Values are entered and displayed as USD through the existing quota conversion
helpers. The plan table and wallet plan cards show only enabled limits.

User subscription cards show current usage for each enabled window. This keeps
the migrated PAR behavior inspectable instead of hiding it behind a single total.

## PAR Import

After the schema and UI are verified, add an idempotent local import command that
accepts a JSON catalog. It creates or updates plans by a stable external key and
does not connect directly to the PAR production database.

Initial mapping:

| PAR | New API |
|---|---|
| `name` | `title` |
| `price_cny` | `price_amount`, `currency=CNY` |
| fixed 30 days | `duration_unit=day`, `duration_value=30` |
| `window_cost_limit` | `window_amount` |
| `window_minutes` | `window_minutes` |
| `daily_cost_limit` | `daily_amount` |
| `weekly_cost_limit` | `weekly_amount` |
| `rate_limit_rpm` | `rpm_limit` |
| `allowed_models` | `allowed_models` |

The current controller forcibly stores `currency=USD`; this implementation removes
that override and validates the supported currency allowlist. CatFK settlement is
still a separate slice, but the plan catalog must display the correct CNY prices.

## Validation

Backend tests must cover:

- Plan validation for negative limits and missing rolling-window duration.
- Currency validation and CNY form round trips.
- SQLite AutoMigrate compatibility for all new columns.
- Existing plans and subscriptions retaining old behavior with zero-value fields.
- Rolling, daily, and weekly exhaustion independently and in combination.
- Lazy reset boundaries for rolling windows, midnight, and Monday.
- Model allowlist acceptance and rejection.
- Refund and post-consume deltas updating every charged counter exactly once.
- Two concurrent requests not exceeding an enabled limit.
- RPM enforcement and expiry of its minute window.

Frontend validation must cover plan-form serialization, default values, editing an
existing plan, and rendering enabled limits. Run `go test ./...`, `bun run
build:check`, frontend tests, lint, and format checks before completion.

## Delivery Order

1. Add failing backend model and validation tests.
2. Add plan and subscription snapshot fields.
3. Implement multi-window pre-consume/refund/post-consume accounting.
4. Add RPM and model allowlist enforcement.
5. Add CNY support, admin form fields, and user usage display.
6. Seed the six PAR-compatible local plans for manual verification.
7. Design the separate CNY and CatFK payment slice.
