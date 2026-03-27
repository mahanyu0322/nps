package file

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
)

// BlacklistEntry 表示黑名单中的一个条目
type BlacklistEntry struct {
	IP             string      `json:"ip"`              // IP地址
	AddTime        time.Time   `json:"add_time"`        // 添加时间
	Reason         string      `json:"reason"`          // 原因
	ExpireTime     time.Time   `json:"expire_time"`     // 过期时间，如果是零值则表示永久
	ConnectionType string      `json:"connection_type"` // 连接类型：ssh, rdp, all等
	Count          int         `json:"count"`           // 触发次数
	AttemptTimes   []time.Time `json:"attempt_times"`   // 尝试时间列表，用于时间窗口计算
}

// BlacklistConfig 黑名单配置
type BlacklistConfig struct {
	Enabled      bool `json:"enabled"`       // 是否启用黑名单
	SSHEnabled   bool `json:"ssh_enabled"`   // 是否对SSH连接启用
	RDPEnabled   bool `json:"rdp_enabled"`   // 是否对RDP连接启用
	HTTPEnabled  bool `json:"http_enabled"`  // 是否对HTTP连接启用
	OtherEnabled bool `json:"other_enabled"` // 是否对其他类型连接启用

	// 触发规则
	SSHThreshold   int `json:"ssh_threshold"`   // SSH连接阈值 (次数/分钟)
	RDPThreshold   int `json:"rdp_threshold"`   // RDP连接阈值
	HTTPThreshold  int `json:"http_threshold"`  // HTTP连接阈值
	OtherThreshold int `json:"other_threshold"` // 其他连接阈值
	
	// 时间窗口（分钟），指定在多少分钟内的连接尝试计入阈值判断
	TimeWindow int `json:"time_window"`

	// 黑名单时长（分钟，0表示永久）
	BlacklistTime int `json:"blacklist_time"`
	
	// 白名单IPs，这些IP不会受到任何限制
	WhitelistIPs []string `json:"whitelist_ips"`
}

// Blacklist 黑名单系统
type Blacklist struct {
	Entries map[string]*BlacklistEntry `json:"entries"` // 黑名单列表，IP为key
	Config  BlacklistConfig            `json:"config"`  // 黑名单配置
	sync.RWMutex
}

// NewBlacklist 创建新的黑名单实例
func NewBlacklist() *Blacklist {
	return &Blacklist{
		Entries: make(map[string]*BlacklistEntry),
		Config: BlacklistConfig{
			Enabled:       true,
			SSHEnabled:    true,
			RDPEnabled:    true,
			HTTPEnabled:   true,
			OtherEnabled:  true,
			SSHThreshold:  5,
			RDPThreshold:  5,
			HTTPThreshold: 10,
			OtherThreshold: 10,
			TimeWindow:    1,
			BlacklistTime: 1440, // 默认24小时
			WhitelistIPs:  []string{}, // 默认空白名单
		},
		RWMutex: sync.RWMutex{},
	}
}

// LoadFromFile 从文件加载黑名单
func (b *Blacklist) LoadFromFile(path string) error {
	b.Lock()
	defer b.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 如果文件不存在，使用默认配置
		return nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	var blacklist Blacklist
	if err = json.Unmarshal(data, &blacklist); err != nil {
		return err
	}

	// 清理过期条目
	now := time.Now()
	for ip, entry := range blacklist.Entries {
		if !entry.ExpireTime.IsZero() && now.After(entry.ExpireTime) {
			delete(blacklist.Entries, ip)
		}
	}

	b.Entries = blacklist.Entries
	b.Config = blacklist.Config
	return nil
}

// SaveToFile 保存黑名单到文件
func (b *Blacklist) SaveToFile(path string) error {
	b.Lock()
	defer b.Unlock()

	// 清理过期条目
	now := time.Now()
	for ip, entry := range b.Entries {
		if !entry.ExpireTime.IsZero() && now.After(entry.ExpireTime) {
			delete(b.Entries, ip)
		}
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logs.Error("创建目录失败: %v", err)
		return err
	}

	// 检查文件是否可写
	var fileExists bool
	_, err := os.Stat(path)
	if err == nil {
		fileExists = true
	}

	// 确保文件可写
	if fileExists {
		// 尝试打开文件以确认写入权限
		testFile, err := os.OpenFile(path, os.O_WRONLY, 0644)
		if err != nil {
			logs.Error("无法写入文件 %s: %v", path, err)
			return err
		}
		testFile.Close()
	}

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		logs.Error("序列化黑名单数据失败: %v", err)
		return err
	}

	// 安全地写入文件（先写入临时文件，再重命名）
	tempPath := path + ".tmp"
	if err := ioutil.WriteFile(tempPath, data, 0644); err != nil {
		logs.Error("写入临时文件失败: %v", err)
		return err
	}

	if err := os.Rename(tempPath, path); err != nil {
		logs.Error("重命名文件失败: %v", err)
		return err
	}

	logs.Info("黑名单配置已成功保存到 %s", path)
	return nil
}

// AddToBlacklist 添加IP到黑名单
func (b *Blacklist) AddToBlacklist(ip, reason, connType string, permanent bool) {
	b.Lock()
	defer b.Unlock()

	// 如果原因为空，设置默认原因
	if reason == "" {
		reason = "手动添加到黑名单"
	}

	var expireTime time.Time
	if !permanent && b.Config.BlacklistTime > 0 {
		expireTime = time.Now().Add(time.Minute * time.Duration(b.Config.BlacklistTime))
	}

	// 检查是否已存在此IP的记录
	if entry, exists := b.Entries[ip]; exists && !permanent {
		// 更新现有记录
		entry.Reason = reason
		if !expireTime.IsZero() {
			entry.ExpireTime = expireTime
		}
		
		// 如果是手动添加，确保不会被误认为是普通连接记录
		if !strings.HasPrefix(reason, "频繁") && !strings.HasPrefix(reason, "连接记录") {
			// 是手动添加，设置一个明确的标志
			entry.Reason = "[手动添加] " + reason
		}
		
		logs.Info("更新IP %s 的黑名单记录，原因: %s, 连接类型: %s, 过期时间: %v", 
			ip, entry.Reason, entry.ConnectionType, entry.ExpireTime)
	} else {
		// 创建新记录
		// 如果是手动添加，确保不会被误认为是普通连接记录
		if !strings.HasPrefix(reason, "频繁") && !strings.HasPrefix(reason, "连接记录") {
			// 是手动添加，设置一个明确的标志
			reason = "[手动添加] " + reason
		}
		
		b.Entries[ip] = &BlacklistEntry{
			IP:             ip,
			AddTime:        time.Now(),
			Reason:         reason,
			ExpireTime:     expireTime,
			ConnectionType: connType,
			Count:          1,
			AttemptTimes:   []time.Time{},
		}
		
		logs.Info("已添加IP %s 到黑名单，原因: %s, 连接类型: %s, 过期时间: %v", 
			ip, reason, connType, expireTime)
	}
}

// RemoveFromBlacklist 从黑名单移除IP
func (b *Blacklist) RemoveFromBlacklist(ip string) {
	b.Lock()
	defer b.Unlock()

	delete(b.Entries, ip)
}

// IsBlacklisted 检查IP是否在黑名单中
func (b *Blacklist) IsBlacklisted(ip, connType string) bool {
	// 首先检查IP是否在白名单中，如果在则直接放行
	if b.IsWhitelisted(ip) {
		return false
	}
	
	b.RLock()
	defer b.RUnlock()

	// 如果黑名单未启用，直接返回false
	if !b.Config.Enabled {
		return false
	}

	// 根据连接类型检查是否启用
	switch connType {
	case "ssh":
		if !b.Config.SSHEnabled {
			return false
		}
	case "rdp":
		if !b.Config.RDPEnabled {
			return false
		}
	case "http":
		if !b.Config.HTTPEnabled {
			return false
		}
	default:
		if !b.Config.OtherEnabled {
			return false
		}
	}

	// 查找IP
	entry, found := b.Entries[ip]
	if !found {
		return false
	}

	// 检查连接类型
	if entry.ConnectionType != "all" && entry.ConnectionType != connType {
		return false
	}

	// 检查是否过期
	if !entry.ExpireTime.IsZero() && time.Now().After(entry.ExpireTime) {
		// 将在下次保存时删除此过期条目
		return false
	}
	
	// 关键修改：检查是否设置了过期时间（表示已达到阈值）
	// 只有设置了过期时间的条目才被视为已加入黑名单
	if entry.ExpireTime.IsZero() {
		// 无过期时间可能表示：
		// 1. 永久黑名单（手动添加的情况）
		// 2. 仅是记录尚未达到阈值
		// 检查Reason字段来区分
		if entry.Reason == "连接记录初始化" || strings.HasPrefix(entry.Reason, "连接记录") {
			// 这只是一个连接记录，尚未达到阈值
			logs.Info("IP %s 有记录但尚未达到黑名单阈值，连接类型: %s, 计数: %d", 
				ip, entry.ConnectionType, entry.Count)
			return false
		}
	}

	// 检查通过，IP确实在黑名单中
	logs.Info("确认IP %s 在黑名单中，原因: %s", ip, entry.Reason)
	return true
}

// RecordConnection 记录连接尝试，如果触发阈值则加入黑名单
func (b *Blacklist) RecordConnection(ip string, connType string) bool {
	// 首先检查IP是否在白名单中，如果在则直接放行
	if b.IsWhitelisted(ip) {
		logs.Info("IP %s 在白名单中，允许连接，不记录", ip)
		return false
	}
	
	// 如果已经在黑名单中，直接返回true（表示应该拒绝）
	if b.IsBlacklisted(ip, connType) {
		logs.Info("IP %s 已在黑名单中，连接类型: %s", ip, connType)
		return true
	}

	b.Lock()
	defer b.Unlock()

	// 如果黑名单功能未启用，直接返回false
	if !b.Config.Enabled {
		logs.Info("黑名单功能未启用，允许IP %s 的连接", ip)
		return false
	}

	// 检查连接类型是否被监控
	var threshold int
	switch connType {
	case "ssh":
		if !b.Config.SSHEnabled {
			logs.Info("SSH连接黑名单未启用，允许IP %s 的连接", ip)
			return false
		}
		threshold = b.Config.SSHThreshold
	case "rdp":
		if !b.Config.RDPEnabled {
			logs.Info("RDP连接黑名单未启用，允许IP %s 的连接", ip)
			return false
		}
		threshold = b.Config.RDPThreshold
	case "http":
		if !b.Config.HTTPEnabled {
			logs.Info("HTTP连接黑名单未启用，允许IP %s 的连接", ip)
			return false
		}
		threshold = b.Config.HTTPThreshold
	default:
		if !b.Config.OtherEnabled {
			logs.Info("其他类型连接黑名单未启用，允许IP %s 的连接", ip)
			return false
		}
		threshold = b.Config.OtherThreshold
	}

	logs.Info("当前黑名单配置：连接类型 %s 的阈值为 %d", connType, threshold)

	// 从配置中获取时间窗口（分钟）
	timeWindowMinutes := b.Config.TimeWindow
	if timeWindowMinutes <= 0 {
		timeWindowMinutes = 1 // 默认为1分钟
	}
	timeWindow := time.Duration(timeWindowMinutes) * time.Minute
	now := time.Now()
	
	logs.Info("使用 %d 分钟的时间窗口进行计数", timeWindowMinutes)

	// 创建或更新IP的连接记录
	entry, exists := b.Entries[ip]
	if !exists {
		// 首次记录此IP
		b.Entries[ip] = &BlacklistEntry{
			IP:             ip,
			AddTime:        now,
			ConnectionType: connType,
			Count:          1, // 首次记录，设置计数为1
			Reason:         "连接记录初始化", 
			AttemptTimes:   []time.Time{now}, // 记录当前尝试时间
		}
		logs.Info("首次记录IP %s 的 %s 连接尝试，当前计数: 1/%d", ip, connType, threshold)
	} else {
		// 已有记录，检查连接类型是否匹配
		if entry.ConnectionType != connType && entry.ConnectionType != "all" {
			// 不同类型的连接尝试，记录为新的类型或更新为"all"
			oldType := entry.ConnectionType
			if entry.ExpireTime.IsZero() || time.Now().Before(entry.ExpireTime) {
				// 如果原记录未过期，则更新为"all"类型
				entry.ConnectionType = "all"
				logs.Info("IP %s 有多种类型连接尝试(%s, %s)，更新为通用类型", ip, oldType, connType)
			} else {
				// 原记录已过期，替换为新类型
				entry.ConnectionType = connType
				entry.AttemptTimes = []time.Time{} // 清空尝试时间列表
				logs.Info("IP %s 的旧记录(%s)已过期，重新记录为 %s 类型", ip, oldType, connType)
			}
		}
		
		// 记录当前尝试时间
		entry.AttemptTimes = append(entry.AttemptTimes, now)
		
		// 清理时间窗口外的记录
		windowStart := now.Add(-timeWindow)
		validAttempts := make([]time.Time, 0)
		for _, t := range entry.AttemptTimes {
			if t.After(windowStart) {
				validAttempts = append(validAttempts, t)
			}
		}
		entry.AttemptTimes = validAttempts
		
		// 更新计数为时间窗口内的尝试次数
		entry.Count = len(entry.AttemptTimes)
		logs.Info("IP %s 的 %s 连接尝试在 %d 分钟内的计数: %d/%d", 
			ip, connType, int(timeWindow.Minutes()), entry.Count, threshold)
	}

	// 获取更新后的entry
	entry = b.Entries[ip]
	
	// 检查是否达到阈值
	if entry.Count >= threshold {
		// 达到阈值，设置黑名单
		var expireTime time.Time
		if b.Config.BlacklistTime > 0 {
			expireTime = time.Now().Add(time.Minute * time.Duration(b.Config.BlacklistTime))
		}
		entry.ExpireTime = expireTime
		
		// 设置更详细的黑名单原因
		var reasonDetail string
		switch connType {
		case "ssh":
			reasonDetail = "SSH连接"
		case "rdp":
			reasonDetail = "远程桌面连接"
		case "http":
			reasonDetail = "HTTP连接"
		default:
			reasonDetail = connType + "类型连接"
		}
		
		timeWindowMinutes := int(timeWindow.Minutes())
		entry.Reason = fmt.Sprintf("频繁%s尝试 (在%d分钟内达到阈值: %d/%d)",
			reasonDetail, timeWindowMinutes, entry.Count, threshold)
			   
		logs.Warn("IP %s 已被加入黑名单，原因: %s, 过期时间: %v", 
			ip, entry.Reason, expireTime)
		return true
	}

	return false
}

// UpdateConfig 更新黑名单配置
func (b *Blacklist) UpdateConfig(config BlacklistConfig) {
	b.Lock()
	defer b.Unlock()
	
	b.Config = config
}

// GetConfig 获取黑名单配置
func (b *Blacklist) GetConfig() BlacklistConfig {
	b.RLock()
	defer b.RUnlock()
	
	return b.Config
}

// GetEntries 获取黑名单条目
func (b *Blacklist) GetEntries() map[string]*BlacklistEntry {
	b.RLock()
	defer b.RUnlock()
	
	// 清理过期条目并返回副本
	now := time.Now()
	entries := make(map[string]*BlacklistEntry)
	
	for ip, entry := range b.Entries {
		// 只返回真正在黑名单中的条目:
		// 1. 有过期时间(表示已达到阈值)，且尚未过期
		// 2. 或者是永久黑名单条目(无过期时间但不是连接记录)
		if (entry.ExpireTime.IsZero() && entry.Reason != "连接记录初始化" && !strings.HasPrefix(entry.Reason, "连接记录")) || 
		   (!entry.ExpireTime.IsZero() && now.Before(entry.ExpireTime)) {
			// 创建条目的副本
			entryCopy := *entry
			entries[ip] = &entryCopy
		}
	}
	
	return entries
}

// IsWhitelisted 检查IP是否在白名单中
func (b *Blacklist) IsWhitelisted(ip string) bool {
	b.RLock()
	defer b.RUnlock()
	
	// 如果白名单为空，直接返回false
	if len(b.Config.WhitelistIPs) == 0 {
		return false
	}
	
	// 检查IP是否在白名单中
	for _, whiteIP := range b.Config.WhitelistIPs {
		if whiteIP == ip {
			logs.Info("IP %s 在白名单中，允许通过", ip)
			return true
		}
	}
	
	return false
}

// AddToWhitelist 添加IP到白名单
func (b *Blacklist) AddToWhitelist(ip string) {
	b.Lock()
	defer b.Unlock()
	
	// 检查IP是否已在白名单中
	for _, whiteIP := range b.Config.WhitelistIPs {
		if whiteIP == ip {
			// 已经存在，无需添加
			logs.Info("IP %s 已经在白名单中", ip)
			return
		}
	}
	
	// 添加到白名单
	b.Config.WhitelistIPs = append(b.Config.WhitelistIPs, ip)
	
	// 如果IP在黑名单中，从黑名单移除
	delete(b.Entries, ip)
	
	logs.Info("已添加IP %s 到白名单", ip)
}

// RemoveFromWhitelist 从白名单中移除IP
func (b *Blacklist) RemoveFromWhitelist(ip string) {
	b.Lock()
	defer b.Unlock()
	
	// 从白名单中移除IP
	for i, whiteIP := range b.Config.WhitelistIPs {
		if whiteIP == ip {
			// 找到IP，移除它
			b.Config.WhitelistIPs = append(b.Config.WhitelistIPs[:i], b.Config.WhitelistIPs[i+1:]...)
			logs.Info("已从白名单中移除IP %s", ip)
			return
		}
	}
	
	logs.Info("IP %s 不在白名单中", ip)
}

// GetWhitelistIPs 获取白名单IPs
func (b *Blacklist) GetWhitelistIPs() []string {
	b.RLock()
	defer b.RUnlock()
	
	// 返回白名单副本
	whitelist := make([]string, len(b.Config.WhitelistIPs))
	copy(whitelist, b.Config.WhitelistIPs)
	
	return whitelist
}

// GetBlacklistedEntries 获取确实已被加入黑名单的条目
func (b *Blacklist) GetBlacklistedEntries() map[string]*BlacklistEntry {
	entries := b.GetEntries()
	logs.Info("获取黑名单条目，共 %d 条真实黑名单条目", len(entries))
	return entries
}

// GetAllEntries 获取所有记录，包括连接记录和黑名单条目(管理用)
func (b *Blacklist) GetAllEntries() map[string]*BlacklistEntry {
	b.RLock()
	defer b.RUnlock()
	
	// 清理过期条目并返回副本
	now := time.Now()
	entries := make(map[string]*BlacklistEntry)
	
	for ip, entry := range b.Entries {
		if entry.ExpireTime.IsZero() || now.Before(entry.ExpireTime) {
			// 创建条目的副本
			entryCopy := *entry
			entries[ip] = &entryCopy
		}
	}
	
	logs.Info("获取所有记录，包括连接记录和黑名单，共 %d 条", len(entries))
	return entries
} 