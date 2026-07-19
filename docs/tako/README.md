# Tako 迁移工作清单

> 本次工作:把 PAR + Sub2API 整套迁移到 new-api(重构为 tako)+ 新 sub2api(纯账号池)。本文件是**事项清单**(任务 / 进度 / 待决 / 待查),供云端持续迭代。
> 详细方案与背景:`my-documents` 知识库 `roadmap/tako-new-api-migration.md`;PAR 现状:`my-documents/services/par.md`。

## 背景(一句话)

new-api 已覆盖 PAR 套餐/用户/key 体系绝大部分(订阅状态机 / 窗口重置 / 余额回退 / 分组升降级 / 多 OAuth / 多 token / channel 指向 sub2api / 兑换码),**改造 ≈ 配置 + 数据迁移 + 少量适配**,不是从零开发。

## 本次要做的事情

### ✅ 已完成(2026-07-20)
- [x] clone new-api → `~/develop/new-api-src`
- [x] fork → `Barrierml/new-api`
- [x] 改造分支 `tako`(基于上游 main)
- [x] 本工作清单文档 `docs/tako/README.md`

### 🔄 阶段一:套餐 + 登录 + key(用户优先,先做)
- [ ] **开发环境**:`new-api-src` 用 PostgreSQL 跑起来 → `system_name=tako` + 前端 logo/标题 → 套餐管理 UI 点通
- [ ] **套餐**:建 5 档 Pro `SubscriptionPlan`(¥59/119/199/329/499,month 周期)+ 按量 metered 默认;`multiplier` 0.1 → model ratio 换算;5h 滑窗 → `QuotaResetPeriod=custom`(`custom_seconds=18000`)
- [ ] **登录**:argon2id 旧密码兼容(改 `common.ValidatePasswordAndHash`)+ Google(OIDC)+ 邮箱验证码免密登录端点;迁移 `par_users`
- [ ] **key**:`cr_` 前缀(改生成 or 全量换发)+ 迁移 `par_keys`;单 active → 多 token 放宽

### 📋 阶段二:后端迁移
- [ ] channel 指向新 sub2api(`BaseURL`+`ModelMapping`+`Group`)
- [ ] 计费口径对齐(quota / `pkg/billingexpr` 表达窗口 + 倍率)
- [ ] 模型目录/定价(`par_models` → new-api 模型 + ratio)

### 📋 阶段三:新 sub2api 瘦身
- [ ] 剥离用户/计费/余额,只留 OAuth 账号池/调度/reauth/协议桥接(或新建纯账号池实例)

### 📋 阶段四:数据迁移 + 割接
- [ ] OAuth 账号池 → 新 sub2api
- [ ] 用户/余额/套餐/订阅/usage → new-api
- [ ] 下游适配:Happy Server / ReplyHub / COI / KAB / Codex / 视频纪要 / 外部客户端
- [ ] 灰度切流 + 回滚(PAR 保留)

### ⏳ 待决策(需拍板)
- [ ] tako-cli 集成:new-api fork 作为 submodule 进 `tako-cli/new-api/`(tako-cli 当前 `feature/kiro-login-gui` 工作区有 WIP,待清理后接)
- [ ] 支付渠道:CatFK 兑换码 vs EPay 在线
- [ ] 新 sub2api:瘦身现有 vs 新建实例
- [ ] `cr_` 前缀:改 token 生成 vs 迁移时全量换发

### 🔍 待技术确认(我来查)
- [ ] `common.Password2Hash` / `ValidatePasswordAndHash` 是否支持 argon2id(否则迁移期双验 + rehash)
- [ ] `controller/subscription.go` 的 admin API 端点(套餐管理 CRUD)
- [ ] oauth Google via OIDC 接法

## 原则

1. 在 new-api 已有能力上改,不重写
2. 品牌 tako 叠加(`system_name` + 前端),不动代码层 new-api/QuantumNous 版权标识
3. 遵守 `AGENTS.md`(`common.*` JSON / 三库 migration / `quota_math.go` / testify)
4. DB 用 PostgreSQL(跟 PAR pgsql15 一致)

## 参考

- 跨项目方案:`my-documents/roadmap/tako-new-api-migration.md`
- PAR 现状:`my-documents/services/par.md`
- new-api 规范:本仓库 `AGENTS.md`
- 决策背景:`my-documents/changelog/2026-07-20.md`
