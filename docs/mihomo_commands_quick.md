# Mihomo 最常用 20 条命令（精简版）

> 适合日常运维快速复制执行。  
> 完整版请看：`docs/mihomo_commands.md`

## 1) 版本与状态

```bash
/usr/local/bin/mihomo -v
```

```bash
systemctl status mihomo --no-pager
```

## 2) 启停与重启

```bash
systemctl start mihomo
```

```bash
systemctl stop mihomo
```

```bash
systemctl restart mihomo
```

## 3) 开机自启

```bash
systemctl enable mihomo
```

```bash
systemctl disable mihomo
```

## 4) 配置文件操作

```bash
ls -lh /etc/mihomo/config.yaml
```

```bash
cp /etc/mihomo/config.yaml /etc/mihomo/config.yaml.bak.$(date +%F_%H%M%S)
```

```bash
curl -L "<你的订阅URL>" -o /etc/mihomo/config.yaml
```

```bash
/usr/local/bin/mihomo -t -d /etc/mihomo -f /etc/mihomo/config.yaml
```

## 5) 日志与端口

```bash
journalctl -u mihomo -f
```

```bash
journalctl -u mihomo -n 100 --no-pager
```

```bash
ss -lntp | grep mihomo
```

## 6) 代理可用性测试

```bash
curl -x http://127.0.0.1:7890 https://ipinfo.io/ip
```

```bash
http_proxy=http://127.0.0.1:7890 https_proxy=http://127.0.0.1:7890 curl https://ipinfo.io/ip
```

## 7) 自动更新（你当前已配置）

```bash
cat /etc/mihomo/subscription.url
```

```bash
echo "<你的订阅URL>" > /etc/mihomo/subscription.url
```

```bash
chmod 600 /etc/mihomo/subscription.url
```

```bash
systemctl start mihomo-update.service
```

```bash
systemctl status mihomo-update.timer --no-pager
```

```bash
journalctl -u mihomo-update.service -n 100 --no-pager
```

## 8) 出问题时一键排查

```bash
/usr/local/bin/mihomo -t -d /etc/mihomo -f /etc/mihomo/config.yaml && systemctl restart mihomo && systemctl status mihomo --no-pager
```

