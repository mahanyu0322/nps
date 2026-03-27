package controllers

import (
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego/logs"
	"strings"
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

// 获取黑名单列表
func (s *BlacklistController) GetList() {
	// 获取黑名单条目
	entries := file.GetDb().GetBlacklistedEntries()
	list := make([]map[string]interface{}, 0)
	
	for ip, entry := range entries {
		entryMap := make(map[string]interface{})
		entryMap["ip"] = ip
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

// 获取已经过滤的黑名单条目（仅包含真正被拉黑的IP）
func (s *BlacklistController) Index() {
	// 获取已经过滤的黑名单条目（仅包含真正被拉黑的IP）
	blacklist := file.GetDb().GetBlacklistedEntries()
	
	s.Data["title"] = "黑名单管理"
	s.Data["blacklist"] = blacklist
	s.SetInfo("黑名单")
	s.display("blacklist/list")
} 