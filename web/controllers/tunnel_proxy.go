package controllers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"ehang.io/nps/lib/file"
)

const (
	btProxySyncWrapper = "/usr/local/bin/nps-bt-proxy-sync"
)

type tunnelAutoProxyState struct {
	Mode          string
	ServerIp      string
	ProxyDomain   string
	ProxySiteName string
}

type btProxySyncResult struct {
	Status   bool   `json:"status"`
	Msg      string `json:"msg"`
	SiteName string `json:"site_name"`
	SiteID   int    `json:"site_id"`
	Domain   string `json:"domain"`
	Target   string `json:"target"`
}

func captureTunnelAutoProxyState(t *file.Tunnel) *tunnelAutoProxyState {
	if t == nil {
		return nil
	}
	return &tunnelAutoProxyState{
		Mode:          t.Mode,
		ServerIp:      strings.TrimSpace(t.ServerIp),
		ProxyDomain:   strings.TrimSpace(t.ProxyDomain),
		ProxySiteName: strings.TrimSpace(t.ProxySiteName),
	}
}

func syncTunnelAutoProxy(t *file.Tunnel, previous *tunnelAutoProxyState) error {
	if t == nil {
		return nil
	}

	t.ServerIp = strings.TrimSpace(t.ServerIp)
	t.ProxyDomain = strings.TrimSpace(t.ProxyDomain)

	if !shouldManageTunnelProxy(t.Mode, t.ServerIp, t.ProxyDomain) {
		if previous != nil && previous.ProxySiteName != "" {
			if err := deleteManagedTunnelProxy(previous.ProxySiteName); err != nil {
				return err
			}
		}
		t.ProxySiteName = ""
		return nil
	}

	siteName, err := upsertManagedTunnelProxy(t, previous)
	if err != nil {
		return err
	}
	t.ProxySiteName = siteName
	return nil
}

func shouldManageTunnelProxy(mode, serverIP, proxyDomain string) bool {
	return mode == "tcp" && strings.TrimSpace(serverIP) == "127.0.0.1" && strings.TrimSpace(proxyDomain) != ""
}

func upsertManagedTunnelProxy(t *file.Tunnel, previous *tunnelAutoProxyState) (string, error) {
	args := []string{
		"upsert",
		"--domain", t.ProxyDomain,
		"--target", fmt.Sprintf("http://127.0.0.1:%d/", t.Port),
		"--remark", fmt.Sprintf("Managed by nps tunnel #%d", t.Id),
	}
	if previous != nil && previous.ProxySiteName != "" {
		args = append(args, "--site-name", previous.ProxySiteName)
	}

	result, err := runBtProxySync(args...)
	if err != nil {
		return "", err
	}
	if result.SiteName == "" {
		return "", fmt.Errorf("bt proxy sync returned empty site name")
	}
	return result.SiteName, nil
}

func deleteManagedTunnelProxy(siteName string) error {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return nil
	}
	_, err := runBtProxySync("delete", "--site-name", siteName)
	return err
}

func runBtProxySync(args ...string) (*btProxySyncResult, error) {
	commandArgs := append([]string{"-n", btProxySyncWrapper}, args...)
	cmd := exec.Command("/usr/bin/sudo", commandArgs...)
	output, err := cmd.CombinedOutput()
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		if err != nil {
			return nil, fmt.Errorf("bt proxy sync failed: %w", err)
		}
		return nil, fmt.Errorf("bt proxy sync returned empty output")
	}

	result := new(btProxySyncResult)
	if jsonErr := json.Unmarshal(output, result); jsonErr != nil {
		if err != nil {
			return nil, fmt.Errorf("bt proxy sync failed: %s", raw)
		}
		return nil, fmt.Errorf("bt proxy sync returned invalid json: %s", raw)
	}
	if !result.Status {
		if result.Msg == "" {
			result.Msg = "bt proxy sync failed"
		}
		return result, fmt.Errorf(result.Msg)
	}
	return result, nil
}
