维护分支： https://github.com/yinpengfei/komari-agent/tree/maint/1.5.1

当前维护版本：**1.5.3**（14 个平台安装包与 Docker 镜像，见 Release）。

**[安装、升级与维护指南](https://github.com/yinpengfei/komari/blob/maint/1.5.1/INSTALL_MAINTENANCE.md#agent-安装)** · [1.5.3 下载](https://github.com/yinpengfei/komari-agent/releases/tag/1.5.3) · [隐私与更新行为](PRIVACY.md)

默认分支保持 main；请从本维护分支查看维护版文档。1.5.3 默认自动查询缺失的公网 IP：IPv6 优先读取网卡，缺失时才查询 HTTPS ipify；IPv4 通过 HTTPS ipify 查询。可显式关闭外部查询。

# komari-agent

## 配置方式

agent 参数可以通过命令行参数、环境变量或 JSON 配置文件传入。

最小启动示例：

```bash
./komari-agent --endpoint "https://example.com" --token "your-token"
```

使用环境变量：

```bash
export AGENT_ENDPOINT="https://example.com"
export AGENT_TOKEN="your-token"
./komari-agent
```

使用 JSON 配置文件：

```bash
./komari-agent --config ./config.json
```

`config.json` 示例：

```json
{
  "endpoint": "https://example.com",
  "token": "your-token",
  "interval": 3,
  "disable_auto_update": false,
  "disable_web_ssh": false,
  "ignore_unsafe_cert": false
}
```

配置优先级从低到高为：默认值、命令行参数、环境变量、JSON 配置文件。

常用配置项：

表中支持版本表示该参数本身首次在发布 tag 中出现；环境变量和 JSON 配置文件方式从 `1.1.33` 起支持，早于最早 tag 的参数记为 `0.0.9`。

| JSON 字段 | 环境变量 | 命令行参数 | 说明 | 支持版本 |
| --- | --- | --- | --- | --- |
| `endpoint` | `AGENT_ENDPOINT` | `--endpoint`, `-e` | 面板地址 | `0.0.9` |
| `token` | `AGENT_TOKEN` | `--token`, `-t` | agent token | `0.0.9` |
| `interval` | `AGENT_INTERVAL` | `--interval`, `-i` | 数据采集间隔，单位秒 | `0.0.9` |
| `disable_auto_update` | `AGENT_DISABLE_AUTO_UPDATE` | `--disable-auto-update` | 禁用自动更新 | `0.0.9` |
| `disable_web_ssh` | `AGENT_DISABLE_WEB_SSH` | `--disable-web-ssh` | 禁用远程控制 | `0.0.9` |
| `ignore_unsafe_cert` | `AGENT_IGNORE_UNSAFE_CERT` | `--ignore-unsafe-cert`, `-u` | 忽略不安全证书 | `0.0.9` |
| `include_nics` | `AGENT_INCLUDE_NICS` | `--include-nics` | 仅统计指定网卡，逗号分隔 | `0.0.22` |
| `exclude_nics` | `AGENT_EXCLUDE_NICS` | `--exclude-nics` | 排除指定网卡，逗号分隔 | `0.0.22` |
| `include_mountpoints` | `AGENT_INCLUDE_MOUNTPOINTS` | `--include-mountpoint` | 仅统计指定挂载点，分号分隔 | `0.1.0` |
| `month_rotate` | `AGENT_MONTH_ROTATE` | `--month-rotate` | 流量统计每月重置日期，`0` 为禁用 | `0.1.0` |
| `auto_discovery_key` | `AGENT_AUTO_DISCOVERY_KEY` | `--auto-discovery` | 自动发现密钥 | `1.0.40` |
| `custom_dns` | `AGENT_CUSTOM_DNS` | `--custom-dns` | 自定义 DNS 服务器 | `1.0.80` |
| `enable_gpu` | `AGENT_ENABLE_GPU` | `--gpu` | 启用详细 GPU 监控 | `1.0.80` |
| `disable_compression` | `AGENT_DISABLE_COMPRESSION` | `--disable-compression` | 禁用 v2 传输压缩 | `1.2.10` |
| `prefer_ip_version` | `AGENT_PREFER_IP_VERSION` | `--prefer-ip-version` | 优先使用 IP 版本，可选 `4` 或 `6` | 未发布 |
| `allow_external_ip_lookup` | `AGENT_ALLOW_EXTERNAL_IP_LOOKUP` | `--allow-external-ip-lookup` | 允许缺失地址通过 HTTPS ipify 补齐；1.5.3 起默认开启，IPv6 优先网卡；设为 false 禁止外部查询 | 本 Fork 1.5.2 |

完整参数可运行：

```bash
./komari-agent --help
```

详见 `cmd/flags/flag.go` 及 `cmd/root.go`

## 1.5.3 的 IP 获取规则

- `--custom-ipv4` / `--custom-ipv6` 最高优先级，指定哪一种就跳过该地址的自动获取。
- 默认 IPv4 查询 `https://api.ipify.org?format=json`，不使用网卡内网 IPv4 代替公网地址。
- 默认 IPv6 优先读取已启用、非回环网卡的公网候选地址；按网卡名、地址数值排序选择首个候选。沿用网卡筛选规则：`--include-nics` 的显式匹配优先于 `--exclude-nics`，均支持通配符。
- 自动 IPv6 筛选采用保守规则：接受 `2000::/3`，排除 IANA 特殊协议分配 `2001::/23`、文档地址 `2001:db8::/32` 与 `3fff::/20`、6to4 `2002::/16`。私有、链路本地、回环、组播及 IPv4 映射地址不会成为候选。特殊用途地址仍可手动指定。
- 没有候选地址时才查询 `https://api6.ipify.org?format=json`。查询失败留空，下一次基础信息更新重试；启动、重连及默认每 5 分钟更新一次。
- 网卡地址筛选不验证互联网连通性，不保证它与实际出口地址一致；多网卡或 NAT66 场景可手动指定。
- `--get-ip-addr-from-nic` 保留显式网卡模式：可返回私网地址，任意一种地址存在就返回，不补查另一种；手动指定地址仍优先。

关闭所有第三方 IP 查询：添加 `--allow-external-ip-lookup=false`，或设置环境变量 `AGENT_ALLOW_EXTERNAL_IP_LOOKUP=false`，或 JSON 配置 `"allow_external_ip_lookup": false`。关闭后仍读取网卡公网 IPv6；IPv4 需手动指定。配置优先级仍为默认值 < 命令行 < 环境变量 < JSON。

1.5.2 默认关闭查询；升级到 1.5.3 后，未显式配置该开关的节点会启用上述查询。已有环境变量/JSON 中的 false 继续生效；需要启用时请修改优先级最高的实际配置。仅更新 Agent 即可使用，无需升级服务端。
