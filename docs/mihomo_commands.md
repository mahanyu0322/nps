# Mihomo 命令集合（服务器版）

> 适用环境：Ubuntu 22.04 / systemd  
> 当前部署路径：`/usr/local/bin/mihomo`，配置目录：`/etc/mihomo`

## 1. 安装与升级

```bash
# 查看系统架构
uname -m

# 获取 mihomo 版本
/usr/local/bin/mihomo -v

# 下载并安装（示例：使用 ghproxy.net）
curl -L "https://ghproxy.net/https://github.com/MetaCubeX/mihomo/releases/download/v1.19.21/mihomo-linux-amd64-compatible-v1.19.21.gz" -o /opt/mihomo/mihomo.gz
gzip -df /opt/mihomo/mihomo.gz
mv -f /opt/mihomo/mihomo /usr/local/bin/mihomo
chmod +x /usr/local/bin/mihomo
/usr/local/bin/mihomo -v
```

## 2. 配置文件管理

```bash
# 当前生效配置
ls -lh /etc/mihomo/config.yaml

# 备份配置
cp /etc/mihomo/config.yaml /etc/mihomo/config.yaml.bak.$(date +%F_%H%M%S)

# 用订阅链接覆盖配置
curl -L "<你的订阅URL>" -o /etc/mihomo/config.yaml

# 校验配置（强烈建议先校验再重启）
/usr/local/bin/mihomo -t -d /etc/mihomo -f /etc/mihomo/config.yaml
```

## 3. systemd 服务命令

```bash
# 启动 / 停止 / 重启
systemctl start mihomo
systemctl stop mihomo
systemctl restart mihomo

# 开机自启
systemctl enable mihomo

# 取消开机自启
systemctl disable mihomo

# 查看状态
systemctl status mihomo --no-pager
```

## 4. 日志与排障

```bash
# 实时日志
journalctl -u mihomo -f

# 最近 200 行日志
journalctl -u mihomo -n 200 --no-pager

# 查看端口监听（常见：7890、9090）
ss -lntp | grep mihomo

# 检查配置中关键项
grep -nE "port:|socks-port:|mixed-port:|external-controller:|secret:" /etc/mihomo/config.yaml
```

## 5. 代理连通性验证

```bash
# 通过 HTTP 代理查看出口 IP
curl -x http://127.0.0.1:7890 https://ipinfo.io/ip

# 给当前 shell 临时设置代理
export http_proxy=http://127.0.0.1:7890
export https_proxy=http://127.0.0.1:7890
curl https://ipinfo.io/ip

# 取消代理环境变量
unset http_proxy https_proxy
```

## 6. 订阅自动更新（当前已配置）

```bash
# 订阅地址文件
cat /etc/mihomo/subscription.url

# 修改订阅链接
echo "<你的订阅URL>" > /etc/mihomo/subscription.url
chmod 600 /etc/mihomo/subscription.url

# 手动触发一次更新
systemctl start mihomo-update.service

# 查看更新服务执行结果
systemctl status mihomo-update.service --no-pager
journalctl -u mihomo-update.service -n 100 --no-pager

# 查看定时任务状态
systemctl status mihomo-update.timer --no-pager
systemctl list-timers --all | grep mihomo-update
```

## 7. 自动更新相关文件位置

```bash
# 更新脚本
/usr/local/bin/mihomo-update-subscription.sh

# 定时任务
/etc/systemd/system/mihomo-update.service
/etc/systemd/system/mihomo-update.timer
```

## 8. 常见问题快速处理

```bash
# 1) 配置校验失败：先看错误行
/usr/local/bin/mihomo -t -d /etc/mihomo -f /etc/mihomo/config.yaml

# 2) 服务起不来：看服务状态和日志
systemctl status mihomo --no-pager
journalctl -u mihomo -n 100 --no-pager

# 3) 订阅更新后没生效：手动重启
systemctl restart mihomo

# 4) 定时器不执行：重新加载并启用
systemctl daemon-reload
systemctl enable --now mihomo-update.timer
```

## 9. 安全建议

```bash
# 如果 external-controller 只本机使用，建议保持 127.0.0.1
grep -n "external-controller" /etc/mihomo/config.yaml

# 如果需要对外开放 controller，务必设置 secret
grep -n "^secret:" /etc/mihomo/config.yaml
```

