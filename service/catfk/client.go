package catfk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting"
)

// CatFK(云猫寄售)API 客户端。移植自 tools/catfk-cli(Python)。
// 商家 cookie 缓存在内存,过期(401/403 或 code!=1)自动重登。

const (
	origin        = "https://catfk.com"
	loginAPI      = origin + "/merchantApi/user/login"
	cardListAPI   = origin + "/merchantApi/goodsCardStorage/list"
	goodsListAPI  = origin + "/merchantApi/Goods/list"
	payOrderAPI   = origin + "/shopApi/Pay/order"
	payQueryAPI   = origin + "/shopApi/Pay/query"
	userChannelAPI = origin + "/shopApi/Shop/getUserChannel"
	dashboardURL  = origin + "/merchant/dashboard/workplace"
)

// GOODS 映射:goods_key -> 发货类型与值。与 scripts/catfk-checkout.py 保持一致。
// quota 值 = 美元额度 * QuotaPerUnit(¥1=3 美元额度)。
const quotaPerUSD = 500000

type GoodsGrant struct {
	Kind  string // "plan" | "quota"
	Value int64  // plan_id 或 quota 数
}

var Goods = map[string]GoodsGrant{
	"vk898s": {"plan", 2},   // Mini ¥59
	"e0b3y5": {"plan", 3},   // Pro mini ¥119
	"r07y8g": {"plan", 1},   // Pro x1 ¥199
	"uhwx0f": {"plan", 4},   // Pro x2 ¥329
	"bx9j3s": {"plan", 5},   // Pro x3 ¥499
	"snae3x": {"plan", 6},   // Pro x4 ¥749
	"cbcg11": {"quota", 15 * quotaPerUSD},  // ¥5 测试档 (¥1=3)
	"r5ufqm": {"quota", 60 * quotaPerUSD},  // ¥20 -> $60
	"ot5e6z": {"quota", 150 * quotaPerUSD}, // ¥50 -> $150
	"jyq5ae": {"quota", 300 * quotaPerUSD}, // ¥100 -> $300
	"paibsa": {"quota", 600 * quotaPerUSD}, // ¥200 -> $600
}

// 支付方式 -> catfk 渠道 code / fallback id
var channelCodeByPay = map[string]string{"alipay": "AlipayPc", "wechat": "WeixinNative"}
var channelIDFallback = map[string]int{"alipay": 1, "wechat": 4}

var (
	cookieMu    sync.RWMutex
	cachedCookie string

	channelMu    sync.Mutex
	channelCache = map[string]int{}
	channelAt    time.Time
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func commonHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
		"Origin":       origin,
		"Referer":      dashboardURL,
		"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
	}
}

// login 用商家账密登录 merchantApi,提取 Set-Cookie 缓存到内存。
func login() error {
	if setting.CatfkMerchantUser == "" || setting.CatfkMerchantPass == "" {
		return fmt.Errorf("catfk 商家账密未配置(CatfkMerchantUser/Pass)")
	}
	body, _ := json.Marshal(map[string]string{
		"username": setting.CatfkMerchantUser,
		"password": setting.CatfkMerchantPass,
	})
	req, _ := http.NewRequest("POST", loginAPI, bytes.NewReader(body))
	for k, v := range commonHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Referer", origin+"/merchant/login")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("catfk login 网络错误: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Code int `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(raw, &parsed)
	// 拼接 Set-Cookie 里的 name=value
	var parts []string
	for _, c := range resp.Cookies() {
		parts = append(parts, c.Name+"="+c.Value)
	}
	if parsed.Code != 1 || len(parts) == 0 {
		return fmt.Errorf("catfk 登录失败: code=%d msg=%s", parsed.Code, parsed.Msg)
	}
	cookieMu.Lock()
	cachedCookie = strings.Join(parts, "; ")
	cookieMu.Unlock()
	return nil
}

func getCookie() string {
	cookieMu.RLock()
	defer cookieMu.RUnlock()
	return cachedCookie
}

// apiPost 带 cookie POST JSON。needAuth=true 时若 cookie 为空或返回登录态失效会重登一次重试。
func apiPost(url string, payload interface{}, needAuth bool) (map[string]interface{}, error) {
	do := func() (map[string]interface{}, int, error) {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		for k, v := range commonHeaders() {
			req.Header.Set(k, v)
		}
		if needAuth {
			req.Header.Set("Cookie", getCookie())
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("%s 响应非JSON: %.200s", url, string(raw))
		}
		return m, resp.StatusCode, nil
	}
	if needAuth && getCookie() == "" {
		if err := login(); err != nil {
			return nil, err
		}
	}
	m, status, err := do()
	if err != nil {
		return nil, err
	}
	// 登录态失效 -> 重登重试一次
	if needAuth && (status == 401 || status == 403) {
		if err := login(); err != nil {
			return nil, err
		}
		m, _, err = do()
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// GetPayChannelID 把 alipay/wechat 映射为商户的 catfk 渠道 id,缓存 1h,失败用 fallback。
func GetPayChannelID(pay string) (int, error) {
	code, ok := channelCodeByPay[pay]
	if !ok {
		return 0, fmt.Errorf("未知支付方式 %q(want alipay|wechat)", pay)
	}
	channelMu.Lock()
	defer channelMu.Unlock()
	if time.Since(channelAt) > time.Hour || len(channelCache) == 0 {
		if cid, err := refreshChannels(); err == nil && len(cid) > 0 {
			channelCache = cid
			channelAt = time.Now()
		}
	}
	if cid, ok := channelCache[code]; ok {
		return cid, nil
	}
	return channelIDFallback[pay], nil
}

func refreshChannels() (map[string]int, error) {
	// getUserChannel 用 form 编码 + merchant token
	body := "token=" + setting.CatfkMerchantToken
	req, _ := http.NewRequest("POST", userChannelAPI, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Data []struct {
			Code   string `json:"code"`
			Id     int    `json:"id"`
			Status int    `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, ch := range parsed.Data {
		if ch.Status == 1 {
			out[ch.Code] = ch.Id
		}
	}
	return out, nil
}

// CreateOrder 下单,返回 tradeNo 和 payurl。
func CreateOrder(goodsKey string, channelID int, contact string) (tradeNo, payurl string, err error) {
	payload := map[string]interface{}{
		"goods_key":       goodsKey,
		"quantity":        1,
		"coupon_code":     "",
		"channel_id":      channelID,
		"contact":         contact,
		"query_password":  "",
		"select_cards_ids": []interface{}{},
		"extend":          map[string]string{"juuid": ""},
	}
	m, err := apiPost(payOrderAPI, payload, false)
	if err != nil {
		return "", "", err
	}
	code, _ := m["code"].(float64)
	if int(code) != 1 {
		return "", "", fmt.Errorf("catfk 下单失败: %v", m["msg"])
	}
	data, _ := m["data"].(map[string]interface{})
	tradeNo, _ = data["trade_no"].(string)
	payurl, _ = data["payurl"].(string)
	if tradeNo == "" || payurl == "" {
		return "", "", fmt.Errorf("catfk 下单返回缺字段: %v", data)
	}
	return tradeNo, payurl, nil
}

// QueryPaid 查订单是否已支付(Pay/query code==1 视为已付)。
func QueryPaid(tradeNo string) (bool, error) {
	m, err := apiPost(payQueryAPI, map[string]string{"trade_no": tradeNo}, false)
	if err != nil {
		return false, err
	}
	code, _ := m["code"].(float64)
	return int(code) == 1, nil
}

// SoldCards 返回某 goods 已售(status==1)的卡密,排除 exclude 里已归属其他订单的。
// 需要商家登录态(merchantApi)。
func SoldCards(goodsKey string, exclude map[string]bool) ([]string, error) {
	gid, err := goodsID(goodsKey)
	if err != nil {
		return nil, err
	}
	m, err := apiPost(cardListAPI, map[string]interface{}{"goods_id": gid, "status": 1}, true)
	if err != nil {
		return nil, err
	}
	var items []interface{}
	if lst, ok := m["list"].([]interface{}); ok {
		items = lst
	} else if d, ok := m["data"].([]interface{}); ok {
		items = d
	}
	var cards []string
	for _, it := range items {
		c, _ := it.(map[string]interface{})
		if c == nil {
			continue
		}
		st, _ := c["status"].(float64)
		secret, _ := c["secret"].(string)
		if int(st) == 1 && secret != "" && !exclude[secret] {
			cards = append(cards, secret)
		}
	}
	return cards, nil
}

func goodsID(goodsKey string) (int, error) {
	m, err := apiPost(goodsListAPI, map[string]interface{}{}, true)
	if err != nil {
		return 0, err
	}
	var items []interface{}
	if d, ok := m["data"].([]interface{}); ok {
		items = d
	} else if g, ok := m["goods"].([]interface{}); ok {
		items = g
	}
	for _, it := range items {
		g, _ := it.(map[string]interface{})
		if g == nil {
			continue
		}
		if k, _ := g["goods_key"].(string); k == goodsKey {
			id, _ := g["id"].(float64)
			return int(id), nil
		}
	}
	return 0, fmt.Errorf("catfk goods_key %s 未找到", goodsKey)
}

