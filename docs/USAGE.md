# Go-VPC Agent 使用指南

## 概述

Go-VPC Agent 是一个 SD-WAN 客户端代理，通过 WireGuard 实现安全的 P2P 网络连接。

## 系统要求

- Go 1.21+
- 支持的操作系统：Linux、macOS、Windows

## 编译

```bash
# 编译 Agent
go build -o agent ./cmd/agent

# 或使用 make（如果有 Makefile）
make build
```

## 配置

Agent 支持通过配置文件或环境变量进行配置。

### 配置文件示例

创建 `config.yaml`：

```yaml
# Management 服务器配置
management:
  server_addr: "management.example.com:443"
  connect_timeout: 10s
  heartbeat_interval: 30s

# Signal 服务器配置
signal:
  server_addr: "signal.example.com:443"
  connect_timeout: 10s

# WireGuard 配置
wireguard:
  listen_port: 51820
  mtu: 1420

# NAT 探测配置
nat:
  stun_servers:
    - "stun.l.google.com:19302"
    - "stun1.l.google.com:19302"
  probe_timeout: 5s
  probe_interval: 5m

# 数据目录（存储身份密钥等）
data_dir: "~/.go-vpc"
```

### 环境变量

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| `GO_VPC_MANAGEMENT_ADDR` | Management 服务器地址 | `localhost:8080` |
| `GO_VPC_SIGNAL_ADDR` | Signal 服务器地址 | `localhost:8081` |
| `GO_VPC_DATA_DIR` | 数据目录 | `~/.go-vpc` |
| `GO_VPC_WG_PORT` | WireGuard 监听端口 | `51820` |

## 运行

### 基本用法

```bash
# 使用默认配置
./agent

# 指定配置文件
./agent -config /path/to/config.yaml
```

### 启动流程

Agent 启动时会执行以下步骤：

1. **加载/创建身份** - 从 `~/.go-vpc/identity.json` 加载或创建新的设备密钥
2. **NAT 探测** - 检测本机 NAT 类型（Full Cone、Symmetric 等）
3. **连接 Management** - 建立与管理服务器的 gRPC 连接
4. **设备注册** - 向服务器注册设备，获取分配的 VIP
5. **初始化 WireGuard** - 配置 WireGuard 接口
6. **连接 Signal** - 建立信令通道
7. **启动心跳** - 定期向服务器报告状态
8. **同步 Peer** - 订阅并同步 Peer 列表变化

### 日志输出

```
2024/01/15 10:00:00 Agent 启动中... DeviceID: abc123...
2024/01/15 10:00:01 NAT 探测结果: FullCone (203.0.113.5:12345)
2024/01/15 10:00:02 设备注册成功，分配 VIP: 10.100.0.5/24
2024/01/15 10:00:02 Agent 已启动，VIP: 10.100.0.5/24
```

### 优雅退出

发送 `SIGINT` (Ctrl+C) 或 `SIGTERM` 信号：

```bash
# 前台运行时按 Ctrl+C
# 或发送信号
kill -SIGTERM <pid>
```

## 测试

### 运行单元测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示详细输出
go test -v ./...

# 运行特定包的测试
go test -v ./internal/agent/identity/...
go test -v ./internal/agent/nat/...
go test -v ./internal/agent/wireguard/...
go test -v ./internal/agent/config/...

# 运行测试并生成覆盖率报告
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 测试覆盖的模块

| 模块 | 说明 | 测试文件 |
|------|------|----------|
| `identity` | 密钥生成、指纹采集 | `identity_test.go`, `fingerprint_test.go` |
| `nat` | STUN 探测 | `prober_test.go` |
| `wireguard` | WireGuard 引擎 | `engine_test.go` |
| `config` | 配置加载 | `config_test.go` |

### 手动测试

#### 1. 验证身份生成

```bash
# 删除现有身份（如果有）
rm -rf ~/.go-vpc

# 启动 Agent（会自动创建新身份）
./agent

# 检查生成的身份文件
cat ~/.go-vpc/identity.json
```

预期输出（JSON 格式）：
```json
{
  "ed25519_private_key": "base64...",
  "wg_private_key": "base64..."
}
```

#### 2. 验证 NAT 探测

NAT 探测会自动在启动时执行，日志中会显示结果：

```
NAT 探测结果: FullCone (203.0.113.5:12345)
```

可能的 NAT 类型：
- `FullCone` - 完全锥型（最容易穿透）
- `RestrictedCone` - 受限锥型
- `PortRestrictedCone` - 端口受限锥型
- `Symmetric` - 对称型（最难穿透）
- `Unknown` - 无法确定

#### 3. 验证 WireGuard 接口

```bash
# Linux
ip link show wg0
wg show wg0

# macOS
ifconfig utun*
```

## 故障排查

### 常见问题

**问题：连接 Management 服务器失败**
```
连接 Management 服务器失败: dial tcp: lookup management.example.com: no such host
```
- 检查服务器地址是否正确
- 检查网络连接
- 检查 DNS 解析

**问题：设备注册失败**
```
设备注册失败: rpc error: code = Unauthenticated
```
- 检查设备密钥是否正确
- 检查服务器是否允许新设备注册

**问题：WireGuard 初始化失败**
```
初始化 WireGuard 失败: permission denied
```
- Linux：需要 `CAP_NET_ADMIN` 能力或 root 权限
- macOS：需要管理员权限

### 调试模式

设置环境变量启用详细日志：

```bash
GO_VPC_DEBUG=1 ./agent
```

## 架构说明

```
┌─────────────────────────────────────────────────────────────────┐
│                          Agent                                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Identity  │  │   Config    │  │      NAT Prober         │ │
│  │  (密钥管理)  │  │  (配置加载)  │  │     (NAT 探测)          │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────┐  ┌─────────────────────────────┐  │
│  │   Management Client     │  │      Signal Client          │  │
│  │   (注册、心跳、同步)     │  │     (P2P 信令交换)          │  │
│  └─────────────────────────┘  └─────────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 WireGuard Engine                         │   │
│  │            (用户态 WireGuard 实现)                       │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## API 参考

### gRPC 服务

- **Management Service** (`api/proto/management.proto`)
  - `Register` - 设备注册
  - `Heartbeat` - 心跳保活
  - `SyncPeers` - Peer 列表同步

- **Signal Service** (`api/proto/signal.proto`)
  - `Connect` - 双向信令流
  - 支持的信令类型：Offer、Answer、Punch、PunchAck

## 许可证

[待定]
