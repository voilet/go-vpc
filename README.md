# Go-VPC

Go-VPC 是一个基于 WireGuard 的轻量级 SD-WAN 解决方案，支持 NAT 穿透和 P2P 直连。

## 特性

- 🔐 **WireGuard 加密** - 使用 Curve25519 密钥交换，ChaCha20-Poly1305 加密
- 🌐 **NAT 穿透** - 支持 STUN 探测和 UDP 打洞
- 🔄 **P2P 直连** - 设备间直接通信，无需中转
- 📡 **集中管理** - 通过 Management Server 统一管理设备
- 🚀 **用户态实现** - 无需内核模块，跨平台支持

## 架构

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│     Agent A     │────▶│  Management     │◀────│     Agent B     │
│   (设备 A)      │     │    Server       │     │   (设备 B)      │
└────────┬────────┘     └─────────────────┘     └────────┬────────┘
         │                                               │
         │              ┌─────────────────┐              │
         └─────────────▶│  Signal Server  │◀─────────────┘
                        │   (信令交换)     │
                        └─────────────────┘
                                 │
                        ┌────────▼────────┐
                        │   P2P 直连       │
                        │  (WireGuard)    │
                        └─────────────────┘
```

## 快速开始

### 编译

```bash
go build -o agent ./cmd/agent
```

### 运行

```bash
# 使用默认配置
./agent

# 指定配置文件
./agent -config config.yaml
```

### 配置示例

```yaml
management:
  server_addr: "management.example.com:443"
  heartbeat_interval: 30s

signal:
  server_addr: "signal.example.com:443"

wireguard:
  listen_port: 51820
  mtu: 1420

nat:
  stun_servers:
    - "stun.l.google.com:19302"
```

## 项目结构

```
go-vpc/
├── api/                    # gRPC Proto 定义和生成代码
│   ├── proto/              # .proto 文件
│   ├── management/         # Management 服务生成代码
│   └── signal/             # Signal 服务生成代码
├── cmd/
│   └── agent/              # Agent 入口
├── internal/
│   └── agent/
│       ├── client/         # gRPC 客户端（Management、Signal）
│       ├── config/         # 配置加载
│       ├── identity/       # 密钥管理和设备指纹
│       ├── nat/            # NAT 探测（STUN）
│       ├── route/          # 路由管理
│       └── wireguard/      # WireGuard 用户态引擎
└── docs/
    ├── PRD.md              # 产品需求文档
    ├── USAGE.md            # 使用指南
    └── plans/              # 实现计划
```

## 核心组件

| 组件 | 说明 |
|------|------|
| **Identity** | Ed25519 设备身份 + Curve25519 WireGuard 密钥 |
| **NAT Prober** | STUN 协议探测 NAT 类型 |
| **WireGuard Engine** | wireguard-go 用户态实现 |
| **Management Client** | 设备注册、心跳、Peer 同步 |
| **Signal Client** | P2P 信令交换（Offer/Answer/Punch） |

## 测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test -cover ./...
```

## 文档

- [使用指南](docs/USAGE.md) - 详细的配置和使用说明
- [产品需求](docs/PRD.md) - 产品需求文档
- [实现计划](docs/plans/) - 开发计划文档

## 路线图

- [x] **P0** - Agent 基础架构（密钥、NAT、WireGuard、客户端）
- [ ] **P1** - P2P 打洞实现
- [ ] **P2** - 中继服务（TURN）
- [ ] **P3** - 多网络支持
- [ ] **P4** - 管理控制台

## 许可证

[待定]
