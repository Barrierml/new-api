# Tako:new-api 重构方向

> 把 new-api 重构为 **tako**,替代 PAR + Sub2API 整套。本文件是项目内**方向文档**(探索 / 目的 / 方向),供持续迭代。
> 详细跨项目方案、阶段、数据迁移、待决项:见 `my-documents` 知识库 `roadmap/tako-new-api-migration.md`;PAR 现状:`my-documents/services/par.md`;决策背景:`my-documents/changelog/2026-07-20.md`。

## 探索:new-api 已有的能力

QuantumNous/new-api 是成熟的商业化 LLM 网关(Go + Gin + GORM + React 19),聚合 40+ 上游,带用户/计费/限流/管理台。**关键:它已有的能力覆盖了 PAR 套餐体系的绝大部分,且更完整**——所以重构 ≠ 从零造,而是在已有能力上对齐 PAR 的业务规则。

| 模块 | new-api 已有(源码) |
|---|---|
| 套餐订阅 | `SubscriptionPlan`(周期 year/month/day/custom、额度重置 daily/weekly/monthly/custom、`AllowWalletOverflow`=余额回退、`UpgradeGroup/DowngradeGroup`=分组升降级)、`UserSubscription`(状态机 active/expired/cancelled)、幂等预扣费、订单流、`controller/subscription*.go` |
| 用户/登录 | `User`(密码 bcrypt、多 OAuth:GitHub/Discord/OIDC/微信/Telegram/LinuxDO、邀请码、2FA/Passkey、Casbin) |
| key | `Token`(每用户多 token、配额/过期/`ModelLimits`/`AllowIps`/Group) |
| channel | `Channel`(`BaseURL`+`ModelMapping`+多 key 轮询,可指向 sub2api) |
| 计费 | quota + `pkg/billingexpr`(表达式计费)+ `common/quota_math.go`(防溢出/饱和审计) |
| 支付 | EPay/Stripe/Creem/Waffo + `redemption`(兑换码)+ `topup_*` |
| DB | 三库兼容(SQLite/MySQL/**PostgreSQL**) |

## 目的:为什么 fork 成 tako

PAR(=tako,自研)+ Sub2API 是一套深度定制的 LLM 售卖链路。目标是**用 new-api(重构)替代 PAR 的用户/套餐/key/计费/定价/支付层 + Sub2API 的用户/计费层**;新 sub2api 瘦身为纯 OAuth 账号池。

### 职责重划

| 层 | 旧(PAR + Sub2API) | 新 |
|---|---|---|
| 用户/登录/key/套餐/计费/余额/定价/兑换码/支付 | PAR(部分)+ Sub2API(余额/usage) | **new-api(tako)** 全部承接 |
| OAuth 账号池(ChatGPT/Claude Max/Gemini)/调度/reauth/协议桥接 | Sub2API | **新 sub2api** 只做这层 |

### 解耦红利(新架构带来的简化)

- **pricing feed 依赖消失**:旧架构 Sub2API 消费 PAR `/par/pricing/models.json`+sha256 做**计费**;新架构计费在 new-api,新 sub2api 不管计费→不需要 feed。只剩 KAB(kiro-api-bridge)需适配。
- **OAuth 池保留**:new-api 永不碰 OAuth,通过 `channel` 指向新 sub2api(接线同旧 PAR→Sub2API)。

## 方向:改造原则

1. **在 new-api 已有能力上改,不重写**——套餐/登录/key 基本现成,改造 ≈ 配置 + 数据迁移 + 少量适配。
2. **品牌 tako 叠加**:`system_name` 配置 + 前端 logo/标题;**不动代码层 new-api/QuantumNous 版权标识**(`AGENTS.md` 品牌保护)。
3. **遵守 `AGENTS.md`**:JSON 用 `common.*` wrapper、三库 migration(SQLite/MySQL/PG)、计费安全不变量(`quota_math.go`)、testify 测试。
4. **DB 用 PostgreSQL**(跟 PAR 现有 pgsql15 一致)。

## 阶段(概要,详见 roadmap)

1. **阶段一**:套餐 + 登录 + key(配置 + 少量适配,用户优先)——三块 new-api 基本现成。
2. **阶段二**:channel 指向新 sub2api + 计费口径对齐 + 模型目录/定价迁移。
3. **阶段三**:新 sub2api 瘦身(纯 OAuth 账号池,剥用户/计费)。
4. **阶段四**:数据迁移(OAuth 池→sub2api;用户/余额/套餐→new-api)+ 下游适配(Happy Server / ReplyHub / COI / KAB / Codex / 视频纪要 / 外部客户端)+ 灰度割接。

## 待决项

详见 `my-documents/roadmap/tako-new-api-migration.md`:远程托管、支付渠道(CatFK 兑换码 vs EPay)、新 sub2api 瘦身 vs 新建、`cr_` 前缀改生成 vs 换发、`common.Password2Hash` 的 argon2id 兼容、邮箱验证码免密登录、KAB pricing 适配。

## 参考索引

- 跨项目方案:`my-documents/roadmap/tako-new-api-migration.md`
- PAR 现状(架构/业务模型/下游/规模):`my-documents/services/par.md`
- new-api 开发规范:本仓库 `AGENTS.md`
- 决策背景:`my-documents/changelog/2026-07-20.md`
- PAR 源码:`~/develop/tako-cli/packages/par`(main 分支)
