package setting

// CatFK(云猫寄售)充值渠道配置。商家账密与 token 是敏感信息,只存 options 表(DB),
// 不进代码/仓库。通过 admin 后台配置,热更新(见 model/option.go 的注册与回填)。
var CatfkEnabled = false
var CatfkMerchantUser = ""     // 商家登录用户名(merchantApi 登录,查已售卡密防重兑用)
var CatfkMerchantPass = ""     // 商家登录密码
var CatfkMerchantToken = ""    // shopApi 下单/查支付渠道用的 merchant token
