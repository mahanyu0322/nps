package controllers

import (
	"bytes"
	"context"
	"ehang.io/nps/lib/file"
	"encoding/json"
	"errors"
	"github.com/astaxie/beego/logs"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BlacklistController struct {
	BaseController
}

// 黑名单列表页面
func (s *BlacklistController) List() {
	s.Data["menu"] = "blacklist"
	s.Data["type"] = "blacklist"
	s.display("blacklist/list")
}

func (s *BlacklistController) Proxy() {
	s.Data["menu"] = "blacklist_proxy"
	s.Data["type"] = "blacklist_proxy"
	s.display("blacklist/proxy")
}

// 获取黑名单列表
func (s *BlacklistController) GetList() {
	// 获取黑名单条目
	entries := file.GetDb().GetBlacklistedEntries()
	ipKeyword := strings.TrimSpace(s.getEscapeString("ip"))
	sortedEntries := make([]*file.BlacklistEntry, 0, len(entries))
	for _, entry := range entries {
		sortedEntries = append(sortedEntries, entry)
	}
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].AddTime.After(sortedEntries[j].AddTime)
	})
	list := make([]map[string]interface{}, 0)

	for _, entry := range sortedEntries {
		if ipKeyword != "" && !strings.Contains(entry.IP, ipKeyword) {
			continue
		}
		entryMap := make(map[string]interface{})
		entryMap["ip"] = entry.IP
		entryMap["reason"] = entry.Reason
		entryMap["connection_type"] = entry.ConnectionType
		entryMap["add_time"] = entry.AddTime.Format("2006-01-02 15:04:05")

		if entry.ExpireTime.IsZero() {
			entryMap["expire_time"] = "永久"
		} else {
			entryMap["expire_time"] = entry.ExpireTime.Format("2006-01-02 15:04:05")
		}

		entryMap["count"] = entry.Count

		list = append(list, entryMap)
	}

	s.AjaxTable(list, len(list), len(list), nil)
}

// 添加IP到黑名单
func (s *BlacklistController) Add() {
	if s.Ctx.Request.Method == "GET" {
		// 检查是否有添加参数，如果有则是通过GET请求添加黑名单IP
		if s.GetString("ip") != "" {
			logs.Info("通过GET请求添加黑名单IP: %s, 原因: %s",
				s.GetString("ip"), s.GetString("reason"))

			ip := s.getEscapeString("ip")
			reason := s.getEscapeString("reason")
			connType := s.getEscapeString("conn_type")
			permanent := s.GetString("permanent") == "on"

			file.GetDb().AddToBlacklist(ip, reason, connType, permanent)

			// 重定向到黑名单列表页面
			s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/list", 302)
			return
		}

		// 无参数GET请求，显示添加页面
		s.Data["menu"] = "blacklist"
		s.Data["type"] = "blacklist"
		s.display("blacklist/add")
		return
	}

	// POST请求处理
	ip := s.getEscapeString("ip")
	reason := s.getEscapeString("reason")
	connType := s.getEscapeString("conn_type")
	permanent := s.GetBoolNoErr("permanent")

	file.GetDb().AddToBlacklist(ip, reason, connType, permanent)

	// 检查是否是Ajax请求
	if s.Ctx.Input.IsAjax() {
		// Ajax请求返回JSON
		s.AjaxOk("添加成功")
	} else {
		// 普通表单提交重定向到黑名单列表页面
		s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/list", 302)
	}
}

// 移除IP
func (s *BlacklistController) Del() {
	ip := s.getEscapeString("ip")
	logs.Info("删除黑名单IP: %s", ip)

	file.GetDb().RemoveFromBlacklist(ip)

	// 检查是否是Ajax请求
	if s.Ctx.Input.IsAjax() {
		// Ajax请求返回JSON
		s.AjaxOk("删除成功")
	} else {
		// 普通请求重定向到黑名单列表页面
		s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/list", 302)
	}
}

// 配置页面
func (s *BlacklistController) Config() {
	if s.Ctx.Request.Method == "GET" {
		// 检查是否有配置参数，如果有则是通过GET请求更新配置
		if s.GetString("enabled") != "" || s.GetString("ssh_threshold") != "" {
			logs.Info("通过GET请求更新黑名单配置，参数: enabled=%v, ssh_threshold=%v",
				s.GetString("enabled"), s.GetString("ssh_threshold"))

			// 获取白名单IPs
			whitelistInput := s.GetString("whitelist_ips")
			var whitelistIPs []string
			if whitelistInput != "" {
				// 如果有输入，按逗号分隔解析IP列表
				whitelistIPs = strings.Split(whitelistInput, ",")
				// 去除每个IP两端的空格
				for i, ip := range whitelistIPs {
					whitelistIPs[i] = strings.TrimSpace(ip)
				}
			}

			config := file.BlacklistConfig{
				Enabled:        s.GetString("enabled") == "on",
				SSHEnabled:     s.GetString("ssh_enabled") == "on",
				RDPEnabled:     s.GetString("rdp_enabled") == "on",
				HTTPEnabled:    s.GetString("http_enabled") == "on",
				OtherEnabled:   s.GetString("other_enabled") == "on",
				SSHThreshold:   s.GetIntNoErr("ssh_threshold"),
				RDPThreshold:   s.GetIntNoErr("rdp_threshold"),
				HTTPThreshold:  s.GetIntNoErr("http_threshold"),
				OtherThreshold: s.GetIntNoErr("other_threshold"),
				TimeWindow:     s.GetIntNoErr("time_window"),
				BlacklistTime:  s.GetIntNoErr("blacklist_time"),
				WhitelistIPs:   whitelistIPs,
			}

			logs.Info("解析后的配置: %+v", config)
			file.GetDb().UpdateBlacklistConfig(config)

			// 重定向到黑名单列表页面
			s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/list", 302)
			return
		}

		// 无参数GET请求，显示配置页面
		s.Data["menu"] = "blacklist"
		s.Data["type"] = "blacklist"
		s.Data["config"] = file.GetDb().GetBlacklistConfig()
		s.Data["whitelist_ips"] = strings.Join(file.GetDb().GetWhitelistIPs(), ", ")
		s.display("blacklist/config")
		return
	}

	// POST请求处理
	logs.Info("通过POST请求更新黑名单配置，参数: enabled=%v, ssh_threshold=%v, time_window=%v",
		s.GetString("enabled"), s.GetString("ssh_threshold"), s.GetString("time_window"))

	// 获取白名单IPs
	whitelistInput := s.GetString("whitelist_ips")
	var whitelistIPs []string
	if whitelistInput != "" {
		// 如果有输入，按逗号分隔解析IP列表
		whitelistIPs = strings.Split(whitelistInput, ",")
		// 去除每个IP两端的空格
		for i, ip := range whitelistIPs {
			whitelistIPs[i] = strings.TrimSpace(ip)
		}
	}

	config := file.BlacklistConfig{
		Enabled:        s.GetString("enabled") == "on",
		SSHEnabled:     s.GetString("ssh_enabled") == "on",
		RDPEnabled:     s.GetString("rdp_enabled") == "on",
		HTTPEnabled:    s.GetString("http_enabled") == "on",
		OtherEnabled:   s.GetString("other_enabled") == "on",
		SSHThreshold:   s.GetIntNoErr("ssh_threshold"),
		RDPThreshold:   s.GetIntNoErr("rdp_threshold"),
		HTTPThreshold:  s.GetIntNoErr("http_threshold"),
		OtherThreshold: s.GetIntNoErr("other_threshold"),
		TimeWindow:     s.GetIntNoErr("time_window"),
		BlacklistTime:  s.GetIntNoErr("blacklist_time"),
		WhitelistIPs:   whitelistIPs,
	}

	// 确保时间窗口至少为1分钟
	if config.TimeWindow <= 0 {
		config.TimeWindow = 1
	}

	logs.Info("解析后的配置: %+v", config)
	file.GetDb().UpdateBlacklistConfig(config)

	// 检查是否是Ajax请求
	if s.Ctx.Input.IsAjax() {
		// Ajax请求返回JSON
		s.AjaxOk("配置已更新")
	} else {
		// 普通表单提交重定向到黑名单列表页面
		s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/list", 302)
	}
}

// 添加IP到白名单
func (s *BlacklistController) AddToWhitelist() {
	ip := s.getEscapeString("ip")
	logs.Info("添加IP到白名单: %s", ip)

	file.GetDb().AddToWhitelist(ip)

	// 检查是否是Ajax请求
	if s.Ctx.Input.IsAjax() {
		// Ajax请求返回JSON
		s.AjaxOk("IP已添加到白名单")
	} else {
		// 普通请求重定向到黑名单配置页面
		s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/config", 302)
	}
}

// 从白名单移除IP
func (s *BlacklistController) RemoveFromWhitelist() {
	ip := s.getEscapeString("ip")
	logs.Info("从白名单删除IP: %s", ip)

	file.GetDb().RemoveFromWhitelist(ip)

	// 检查是否是Ajax请求
	if s.Ctx.Input.IsAjax() {
		// Ajax请求返回JSON
		s.AjaxOk("IP已从白名单移除")
	} else {
		// 普通请求重定向到黑名单配置页面
		s.Redirect(s.Data["web_base_url"].(string)+"/blacklist/config", 302)
	}
}

func (s *BlacklistController) MihomoStatus() {
	version, _ := s.mihomoRun(8*time.Second, "/usr/local/bin/mihomo", "-v")
	status, _ := s.mihomoRun(8*time.Second, "systemctl", "status", "mihomo", "--no-pager")
	activeRaw, _ := s.mihomoRun(5*time.Second, "systemctl", "is-active", "mihomo")
	enabledRaw, _ := s.mihomoRun(5*time.Second, "systemctl", "is-enabled", "mihomo")
	timerActiveRaw, _ := s.mihomoRun(5*time.Second, "systemctl", "is-active", "mihomo-update.timer")
	timerEnabledRaw, _ := s.mihomoRun(5*time.Second, "systemctl", "is-enabled", "mihomo-update.timer")
	subscriptionURLBytes, _ := os.ReadFile("/etc/mihomo/subscription.url")
	mode := ""
	respBody, _, err := s.mihomoAPIRequest("GET", "http://127.0.0.1:9090/configs", nil)
	if err == nil {
		var configs map[string]interface{}
		if json.Unmarshal(respBody, &configs) == nil {
			if modeRaw, ok := configs["mode"].(string); ok {
				mode = strings.ToLower(strings.TrimSpace(modeRaw))
			}
		}
	}
	data := map[string]interface{}{
		"version":           strings.TrimSpace(version),
		"status":            status,
		"active":            strings.TrimSpace(activeRaw),
		"enabled":           strings.TrimSpace(enabledRaw),
		"timer_active":      strings.TrimSpace(timerActiveRaw),
		"timer_enabled":     strings.TrimSpace(timerEnabledRaw),
		"auto_update":       strings.TrimSpace(timerEnabledRaw) == "enabled",
		"subscription_url":  strings.TrimSpace(string(subscriptionURLBytes)),
		"mode":              mode,
		"controller_status": s.mihomoControllerStatus(),
	}
	s.mihomoJSON(1, "ok", data)
}

func (s *BlacklistController) MihomoControl() {
	action := strings.TrimSpace(s.getEscapeString("action"))
	actionArgs := map[string][]string{
		"start":   {"start", "mihomo"},
		"stop":    {"stop", "mihomo"},
		"restart": {"restart", "mihomo"},
	}
	args, ok := actionArgs[action]
	if !ok {
		s.mihomoJSON(0, "不支持的操作", nil)
		return
	}
	output, err := s.mihomoRun(20*time.Second, "systemctl", args...)
	if err != nil {
		s.mihomoJSON(0, output, nil)
		return
	}
	activeRaw, _ := s.mihomoRun(5*time.Second, "systemctl", "is-active", "mihomo")
	s.mihomoJSON(1, "操作成功", map[string]interface{}{
		"output": output,
		"active": strings.TrimSpace(activeRaw),
	})
}

func (s *BlacklistController) MihomoValidate() {
	output, err := s.mihomoRun(20*time.Second, "/usr/local/bin/mihomo", "-t", "-d", "/etc/mihomo", "-f", "/etc/mihomo/config.yaml")
	if err != nil {
		s.mihomoJSON(0, output, nil)
		return
	}
	s.mihomoJSON(1, "配置校验通过", map[string]interface{}{"output": output})
}

func (s *BlacklistController) MihomoUpdateSubscription() {
	subscriptionURL := strings.TrimSpace(s.getEscapeString("url"))
	if subscriptionURL == "" || (!strings.HasPrefix(subscriptionURL, "http://") && !strings.HasPrefix(subscriptionURL, "https://")) {
		s.mihomoJSON(0, "订阅链接格式不正确", nil)
		return
	}
	if err := os.WriteFile("/etc/mihomo/subscription.url", []byte(subscriptionURL+"\n"), 0600); err != nil {
		s.mihomoJSON(0, err.Error(), nil)
		return
	}
	output, err := s.mihomoRun(20*time.Second, "systemctl", "start", "mihomo-update.service")
	if err != nil {
		s.mihomoJSON(0, output, nil)
		return
	}
	serviceStatus, _ := s.mihomoRun(8*time.Second, "systemctl", "status", "mihomo-update.service", "--no-pager")
	s.mihomoJSON(1, "已触发订阅更新", map[string]interface{}{
		"output":         output,
		"service_status": serviceStatus,
	})
}

func (s *BlacklistController) MihomoProxies() {
	respBody, _, err := s.mihomoAPIRequest("GET", "http://127.0.0.1:9090/proxies", nil)
	if err != nil {
		s.mihomoJSON(0, err.Error(), nil)
		return
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		s.mihomoJSON(0, err.Error(), nil)
		return
	}
	proxiesRaw, ok := payload["proxies"].(map[string]interface{})
	if !ok {
		s.mihomoJSON(0, "无法解析代理组信息", nil)
		return
	}
	groupNames := make([]string, 0)
	groupData := make(map[string]map[string]interface{})
	for name, v := range proxiesRaw {
		item, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		allRaw, ok := item["all"].([]interface{})
		if !ok || len(allRaw) == 0 {
			continue
		}
		nodes := make([]string, 0, len(allRaw))
		for _, n := range allRaw {
			if nodeName, ok := n.(string); ok {
				nodes = append(nodes, nodeName)
			}
		}
		if len(nodes) == 0 {
			continue
		}
		groupNames = append(groupNames, name)
		groupData[name] = map[string]interface{}{
			"name":  name,
			"now":   item["now"],
			"type":  item["type"],
			"nodes": nodes,
		}
	}
	sort.Strings(groupNames)
	groups := make([]map[string]interface{}, 0, len(groupNames))
	for _, groupName := range groupNames {
		groups = append(groups, groupData[groupName])
	}
	s.mihomoJSON(1, "ok", map[string]interface{}{"groups": groups})
}

func (s *BlacklistController) MihomoSwitchNode() {
	group := strings.TrimSpace(s.getEscapeString("group"))
	node := strings.TrimSpace(s.getEscapeString("node"))
	if group == "" || node == "" {
		s.mihomoJSON(0, "代理组和节点不能为空", nil)
		return
	}
	reqBody, _ := json.Marshal(map[string]string{"name": node})
	_, statusCode, err := s.mihomoAPIRequest("PUT", "http://127.0.0.1:9090/proxies/"+url.PathEscape(group), reqBody)
	if err != nil {
		s.mihomoJSON(0, err.Error(), nil)
		return
	}
	s.mihomoJSON(1, "节点切换成功", map[string]interface{}{
		"group":       group,
		"node":        node,
		"status_code": statusCode,
	})
}

func (s *BlacklistController) MihomoTestProxy() {
	output, err := s.mihomoRun(15*time.Second, "curl", "-sS", "-m", "12", "-x", "http://127.0.0.1:7890", "https://ipinfo.io/ip")
	if err != nil {
		s.mihomoJSON(0, output, nil)
		return
	}
	s.mihomoJSON(1, "代理可用", map[string]interface{}{
		"exit_ip": strings.TrimSpace(output),
		"output":  output,
	})
}

func (s *BlacklistController) MihomoAutoUpdate() {
	enabled := s.GetBoolNoErr("enabled")
	var output string
	var err error
	if enabled {
		output, err = s.mihomoRun(20*time.Second, "systemctl", "enable", "--now", "mihomo-update.timer")
	} else {
		output, err = s.mihomoRun(20*time.Second, "systemctl", "disable", "--now", "mihomo-update.timer")
	}
	if err != nil {
		s.mihomoJSON(0, output, nil)
		return
	}
	timerEnabledRaw, _ := s.mihomoRun(5*time.Second, "systemctl", "is-enabled", "mihomo-update.timer")
	s.mihomoJSON(1, "自动更新设置成功", map[string]interface{}{
		"output":        output,
		"timer_enabled": strings.TrimSpace(timerEnabledRaw),
	})
}

func (s *BlacklistController) MihomoLogs() {
	lines := s.GetIntNoErr("lines")
	if lines <= 0 || lines > 500 {
		lines = 120
	}
	output, err := s.mihomoRun(20*time.Second, "journalctl", "-u", "mihomo", "-n", strconv.Itoa(lines), "--no-pager")
	if err != nil {
		s.mihomoJSON(0, output, nil)
		return
	}
	s.mihomoJSON(1, "ok", map[string]interface{}{"output": output})
}

func (s *BlacklistController) MihomoSwitchMode() {
	mode := strings.ToLower(strings.TrimSpace(s.getEscapeString("mode")))
	if mode != "rule" && mode != "global" && mode != "direct" {
		s.mihomoJSON(0, "模式仅支持 rule/global/direct", nil)
		return
	}
	reqBody, _ := json.Marshal(map[string]string{"mode": mode})
	_, _, err := s.mihomoAPIRequest("PATCH", "http://127.0.0.1:9090/configs", reqBody)
	if err != nil {
		s.mihomoJSON(0, err.Error(), nil)
		return
	}
	s.mihomoJSON(1, "规则模式切换成功", map[string]interface{}{"mode": mode})
}

func (s *BlacklistController) mihomoRun(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	errOutput := strings.TrimSpace(stderr.String())
	if errOutput != "" {
		if output != "" {
			output += "\n"
		}
		output += errOutput
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "命令执行超时", ctx.Err()
	}
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		return output, err
	}
	return output, nil
}

func (s *BlacklistController) mihomoSecret() string {
	content, err := os.ReadFile("/etc/mihomo/config.yaml")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "secret:") {
			continue
		}
		secret := strings.TrimSpace(strings.TrimPrefix(trimmed, "secret:"))
		secret = strings.Trim(secret, `"'`)
		return secret
	}
	return ""
}

func (s *BlacklistController) mihomoControllerStatus() string {
	_, statusCode, err := s.mihomoAPIRequest("GET", "http://127.0.0.1:9090/version", nil)
	if err != nil {
		return "unreachable"
	}
	if statusCode >= 200 && statusCode < 300 {
		return "ok"
	}
	return "error"
}

func (s *BlacklistController) mihomoAPIRequest(method, endpoint string, body []byte) ([]byte, int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	secret := s.mihomoSecret()
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		text := strings.TrimSpace(string(respBody))
		if text == "" {
			text = resp.Status
		}
		return respBody, resp.StatusCode, errors.New(text)
	}
	return respBody, resp.StatusCode, nil
}

func (s *BlacklistController) mihomoJSON(status int, msg string, data interface{}) {
	resp := map[string]interface{}{
		"status": status,
		"msg":    msg,
	}
	if data != nil {
		resp["data"] = data
	}
	s.Data["json"] = resp
	s.ServeJSON()
	s.StopRun()
}

// 获取已经过滤的黑名单条目（仅包含真正被拉黑的IP）
func (s *BlacklistController) Index() {
	// 获取已经过滤的黑名单条目（仅包含真正被拉黑的IP）
	blacklist := file.GetDb().GetBlacklistedEntries()

	s.Data["title"] = "黑名单管理"
	s.Data["blacklist"] = blacklist
	s.SetInfo("黑名单")
	s.display("blacklist/list")
}
