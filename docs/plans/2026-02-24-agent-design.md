# Agent 组件设计文档

> 创建日期：2026-02-24
> 状态：已确认
> 作者：Claude AI + 用户协作

## 1. 概述

Agent 是 Go-VPC 系统的客户端组件，运行在每台需要接入虚拟网络的 x86 服务器上。负责：
- WireGuard 隧道管理
- NAT 探测与穿透
- 本地路由维护
- 与服务端通信（Management / Signal Server）

## 2. 设计决策

| 设计点 | 决策 | 理由 |
|-------|------|------|
| WireGuard 集成 | 混合模式（内核优先，wireguard-go 降级） | 性能最优 + 兼容性保障 |
| NAT 穿透 | 轻量 STUN + 直接打洞 | 简单高效，覆盖 90%+ 场景 |
| 服务端通信 | 全 gRPC（双向流） | 统一技术栈，内置可靠性 |
| 路由管理 | netlink 直接操作 | 简单可靠，调试方便 |
| 漫游处理 | Netlink 监听 + 定时轮询 + 主动通知 | 多重保障，秒级恢复 |
| 配置格式 | YAML | 可读性好，Go 生态支持完善 |
| 部署方式 | systemd 服务 | Linux 标准，管理方便 |

## 3. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                         Agent                               │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Identity    │  │ Config      │  │ State Manager       │  │
│  │ Manager     │  │ Manager     │  │ (peers, routes)     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │              WireGuard Engine (混合模式)                 ││
│  │  ┌─────────────────┐  ┌─────────────────────────────┐  ││
│  │  │ Kernel Module   │  │ wireguard-go (fallback)     │  ││
│  │  │ (Linux 5.6+)    │  │                             │  ││
│  │  └─────────────────┘  └─────────────────────────────┘  ││
│  └─────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ NAT Prober   │  │ Route Manager│  │ Network Watcher  │  │
│  │ (STUN)       │  │ (netlink)    │  │ (IP变化监听)     │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    gRPC Client Layer                    ││
│  │  ┌─────────────────────┐  ┌─────────────────────────┐  ││
│  │  │ Management Client   │  │ Signal Client (stream)  │  ││
│  │  └─────────────────────┘  └─────────────────────────┘  ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

**核心模块：**
- **Identity Manager**：管理设备密钥对和指纹
- **WireGuard Engine**：混合模式的隧道引擎
- **NAT Prober**：STUN 探测和 NAT 类型识别
- **Route Manager**：系统路由表管理
- **Network Watcher**：监听本地 IP 变化，触发漫游处理
- **gRPC Client Layer**：与 Management/Signal Server 通信

## 4. 启动流程

```
  ┌──────────┐
  │  启动    │
  └────┬─────┘
       ▼
  ┌──────────────────┐    否     ┌─────────────────────┐
  │ 检查本地密钥对   │─────────▶│ 生成 Ed25519 密钥对 │
  │ 是否存在？       │           │ + 采集设备指纹      │
  └────────┬─────────┘           └──────────┬──────────┘
           │ 是                              │
           ▼                                 ▼
  ┌──────────────────────────────────────────────────────┐
  │            连接 Management Server (gRPC)             │
  │  - 首次：注册设备（公钥 + 指纹）→ 获取 VIP           │
  │  - 后续：认证 → 获取最新配置（ACL、Relay列表等）     │
  └────────────────────────┬─────────────────────────────┘
                           ▼
  ┌──────────────────────────────────────────────────────┐
  │            初始化 WireGuard Engine                   │
  │  - 检测内核模块可用性 → 选择实现方式                 │
  │  - 创建虚拟网卡（如 wg0）                            │
  │  - 配置本机 VIP 地址                                 │
  └────────────────────────┬─────────────────────────────┘
                           ▼
  ┌──────────────────────────────────────────────────────┐
  │            NAT 探测 (STUN)                           │
  │  - 获取公网 IP:Port                                  │
  │  - 识别 NAT 类型（Full Cone / Restricted / 对称）    │
  └────────────────────────┬─────────────────────────────┘
                           ▼
  ┌──────────────────────────────────────────────────────┐
  │            连接 Signal Server (gRPC Stream)          │
  │  - 建立双向流长连接                                  │
  │  - 上报本机 Endpoint（公网IP:Port + NAT类型）        │
  │  - 进入就绪状态，等待 Peer 握手请求                  │
  └────────────────────────┬─────────────────────────────┘
                           ▼
  ┌──────────────────────────────────────────────────────┐
  │            启动后台服务                              │
  │  - Route Manager：配置 100.64.0.0/10 路由            │
  │  - Network Watcher：监听本地 IP 变化                 │
  │  - 心跳定时器：定期向 Signal Server 发送心跳         │
  └────────────────────────┬─────────────────────────────┘
                           ▼
                    ┌─────────────┐
                    │  运行中     │
                    └─────────────┘
```

## 5. NAT 穿透与 Peer 连接

### 5.1 Peer 连接建立流程

```
    Agent A                    Signal Server                   Agent B
       │                            │                             │
       │  ① 上报 Endpoint           │                             │
       │  (1.2.3.4:51820, NAT类型)  │                             │
       │ ──────────────────────────▶│                             │
       │                            │◀─────────────────────────── │
       │                            │  ① 上报 Endpoint            │
       │                            │  (5.6.7.8:51820, NAT类型)   │
       │                            │                             │
   ════╪════════════════════════════╪═════════════════════════════╪════
       │         A 想要连接 B（首次有流量发往 B 的 VIP）            │
   ════╪════════════════════════════╪═════════════════════════════╪════
       │                            │                             │
       │  ② 请求连接 B              │                             │
       │  (我的公钥, B的VIP)        │                             │
       │ ──────────────────────────▶│                             │
       │                            │  ③ 转发握手请求             │
       │                            │  (A的Endpoint, A的公钥)     │
       │                            │ ───────────────────────────▶│
       │                            │                             │
       │                            │◀─────────────────────────── │
       │                            │  ④ 握手响应                 │
       │  ⑤ 收到 B 的信息          │  (B的Endpoint, B的公钥)     │
       │  (B的Endpoint, B的公钥)    │                             │
       │◀────────────────────────── │                             │
       │                            │                             │
   ════╪════════════════════════════╪═════════════════════════════╪════
       │                   双方开始 UDP 打洞                       │
   ════╪════════════════════════════╪═════════════════════════════╪════
       │                            │                             │
       │  ⑥ WireGuard 握手包 ─────────────────────────────────────▶│
       │◀───────────────────────────────────────────────────────── │
       │                            │                  ⑥ 握手响应 │
       │                            │                             │
       │◀═══════════════════ P2P 隧道建立 ════════════════════════▶│
```

### 5.2 NAT 穿透决策

| A 的 NAT | B 的 NAT | 策略 |
|---------|---------|------|
| 公网/Full | 任意 | 直连 ✓ |
| Restricted | Restricted | 同时打洞 ✓ |
| 对称 | 对称 | 放弃 → Relay |
| 对称 | 锥形 | 尝试打洞，超时→Relay |

**超时与降级：**
- 打洞尝试超时：3 秒
- 超时后自动切换到 Relay 中继
- 后台持续尝试 P2P，成功后自动切回直连

## 6. 拨号漫游处理

### 6.1 IP 变化检测

```
┌──────────────────────────────────────────────────────────────────┐
│                      Network Watcher 模块                         │
├──────────────────────────────────────────────────────────────────┤
│  监听方式（多重保障）：                                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ Netlink 事件    │  │ 定时轮询        │  │ gRPC 连接断开   │  │
│  │ (RTM_NEWADDR    │  │ (每 30s 检查    │  │ (被动感知)      │  │
│  │  RTM_DELADDR)   │  │  本地 IP 列表)  │  │                 │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 6.2 漫游恢复流程

```
  ┌─────────────┐
  │ IP 变化事件 │
  └──────┬──────┘
         │
         ▼ (立即触发，目标 < 1秒)
  ┌──────────────────────┐
  │ 1. 重新 STUN 探测    │
  │    获取新公网 Endpoint│
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ 2. 重连 Signal Server│
  │    (如果连接已断开)  │
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────┐
  │ 3. 上报新 Endpoint   │
  │    到 Signal Server  │
  └──────────┬───────────┘
             │
             ▼
  ┌──────────────────────────────────────────────────────┐
  │ 4. 通知所有活跃 Peer 重新握手                        │
  │    Signal Server 广播 → 各 Peer 更新 Endpoint        │
  └──────────────────────────┬───────────────────────────┘
                             │
                             ▼
  ┌──────────────────────────────────────────────────────┐
  │ 5. WireGuard 自动漫游                                │
  │    - 内置 roaming 能力：收到有效包即更新对端地址     │
  │    - 配合主动通知，实现秒级恢复                      │
  └──────────────────────────────────────────────────────┘
```

**预期恢复时间：** < 1 秒（网络正常情况下）

## 7. 代码结构

```
go-vpc/
├── cmd/
│   └── agent/
│       └── main.go                 # Agent 入口
│
├── internal/
│   └── agent/
│       ├── agent.go                # Agent 主结构，生命周期管理
│       │
│       ├── identity/               # 身份管理模块
│       │   ├── identity.go         # 密钥对生成、加载、存储
│       │   └── fingerprint.go      # 设备指纹采集（CPU+MAC）
│       │
│       ├── wireguard/              # WireGuard 引擎模块
│       │   ├── engine.go           # 混合模式引擎（内核/userspace 切换）
│       │   ├── kernel.go           # 内核模块操作（netlink）
│       │   ├── userspace.go        # wireguard-go 封装
│       │   └── peer.go             # Peer 管理（添加/删除/更新）
│       │
│       ├── nat/                    # NAT 探测模块
│       │   ├── prober.go           # STUN 探测主逻辑
│       │   ├── stun.go             # STUN 协议实现
│       │   └── types.go            # NAT 类型定义
│       │
│       ├── route/                  # 路由管理模块
│       │   └── manager.go          # 系统路由表操作（netlink）
│       │
│       ├── network/                # 网络监控模块
│       │   └── watcher.go          # 本地 IP 变化监听
│       │
│       ├── client/                 # gRPC 客户端模块
│       │   ├── management.go       # Management Server 客户端
│       │   └── signal.go           # Signal Server 客户端（双向流）
│       │
│       └── config/                 # 配置管理模块
│           ├── config.go           # 配置结构定义
│           └── loader.go           # 配置加载（YAML）
│
├── pkg/                            # 可复用的公共包
│   ├── proto/                      # gRPC Proto 定义
│   │   ├── management.proto
│   │   └── signal.proto
│   └── types/                      # 公共类型定义
│       └── endpoint.go             # Endpoint、VIP 等类型
│
├── configs/
│   └── agent.example.yaml          # 配置文件示例
│
└── go.mod
```

## 8. 配置文件

```yaml
# Agent 配置文件
agent:
  # 数据目录（存放密钥、状态等）
  data_dir: /var/lib/go-vpc

  # 日志级别: debug, info, warn, error
  log_level: info

# 服务端连接配置
server:
  # Management Server 地址
  management:
    address: management.example.com:443
    tls: true

  # Signal Server 地址（可配置多个，自动选择延迟最低的）
  signal:
    - address: signal-cn.example.com:443
      region: cn
    - address: signal-us.example.com:443
      region: us

# WireGuard 配置
wireguard:
  # 监听端口（0 表示随机）
  listen_port: 51820

  # 强制使用 userspace 实现（调试用）
  force_userspace: false

  # MTU 设置
  mtu: 1420

# NAT 探测配置
nat:
  # STUN 服务器列表
  stun_servers:
    - stun.l.google.com:19302
    - stun.cloudflare.com:3478

  # 探测超时（秒）
  timeout: 3

# 网络监控配置
network:
  # 轮询间隔（秒），作为 Netlink 监听的补充
  poll_interval: 30

# 连接配置
connection:
  # P2P 打洞超时（秒）
  handshake_timeout: 3

  # 心跳间隔（秒）
  heartbeat_interval: 15

  # 重连退避最大值（秒）
  max_reconnect_backoff: 60
```

## 9. 核心接口

```go
// ==================== Agent 主接口 ====================

// Agent 是客户端代理的主接口
type Agent interface {
    // Start 启动 Agent（阻塞直到收到停止信号）
    Start(ctx context.Context) error

    // Stop 优雅停止 Agent
    Stop() error

    // Status 返回当前状态
    Status() AgentStatus
}

// ==================== WireGuard 引擎接口 ====================

// WireGuardEngine 抽象 WireGuard 操作
type WireGuardEngine interface {
    // Init 初始化引擎，创建虚拟网卡
    Init(cfg WireGuardConfig) error

    // AddPeer 添加对端节点
    AddPeer(peer PeerConfig) error

    // RemovePeer 移除对端节点
    RemovePeer(publicKey string) error

    // UpdatePeerEndpoint 更新对端地址（漫游时使用）
    UpdatePeerEndpoint(publicKey string, endpoint string) error

    // Close 关闭引擎，清理资源
    Close() error
}

// ==================== NAT 探测接口 ====================

// NATProber 负责 NAT 探测
type NATProber interface {
    // Probe 执行 STUN 探测，返回公网地址和 NAT 类型
    Probe(ctx context.Context) (*NATResult, error)
}

// NATResult 探测结果
type NATResult struct {
    PublicAddr string   // 公网 IP:Port
    NATType    NATType  // NAT 类型
}

// NATType NAT 类型枚举
type NATType int

const (
    NATTypeUnknown NATType = iota
    NATTypeNone            // 公网直连
    NATTypeFullCone        // 完全锥形
    NATTypeRestricted      // 受限锥形
    NATTypePortRestricted  // 端口受限锥形
    NATTypeSymmetric       // 对称型
)

// ==================== 网络监控接口 ====================

// NetworkWatcher 监控本地网络变化
type NetworkWatcher interface {
    // Watch 开始监控，IP 变化时通过 channel 通知
    Watch(ctx context.Context) (<-chan NetworkEvent, error)
}

// NetworkEvent 网络变化事件
type NetworkEvent struct {
    Type      EventType
    Interface string
    OldAddr   string
    NewAddr   string
}
```

## 10. gRPC Proto 定义

### 10.1 Management Service

```protobuf
syntax = "proto3";

package govpc.management.v1;

option go_package = "github.com/yourorg/go-vpc/pkg/proto/management";

service ManagementService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Authenticate(AuthRequest) returns (AuthResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc GetPeerInfo(GetPeerInfoRequest) returns (GetPeerInfoResponse);
}

message RegisterRequest {
  string public_key = 1;
  string device_fingerprint = 2;
  string hostname = 3;
  map<string, string> labels = 4;
}

message RegisterResponse {
  string device_id = 1;
  string vip = 2;
  int32 vip_prefix = 3;
  string auth_token = 4;
}

message AuthRequest {
  string device_id = 1;
  string auth_token = 2;
  string public_key = 3;
}

message AuthResponse {
  bool success = 1;
  Config config = 2;
}

message Config {
  repeated RelayServer relays = 1;
  repeated ACLRule acl_rules = 2;
  int64 config_version = 3;
}

message RelayServer {
  string address = 1;
  string region = 2;
  int32 priority = 3;
}

message ACLRule {
  string src_vip = 1;
  string dst_vip = 2;
  string action = 3;
}

message HeartbeatRequest {
  string device_id = 1;
  AgentStatus status = 2;
}

message AgentStatus {
  int64 uptime_seconds = 1;
  int32 active_peers = 2;
  int64 bytes_sent = 3;
  int64 bytes_received = 4;
}

message HeartbeatResponse {
  bool config_updated = 1;
  int64 latest_config_version = 2;
}

message GetPeerInfoRequest {
  string vip = 1;
}

message GetPeerInfoResponse {
  string device_id = 1;
  string public_key = 2;
  repeated string allowed_ips = 3;
}
```

### 10.2 Signal Service

```protobuf
syntax = "proto3";

package govpc.signal.v1;

option go_package = "github.com/yourorg/go-vpc/pkg/proto/signal";

service SignalService {
  rpc Connect(stream SignalMessage) returns (stream SignalMessage);
}

message SignalMessage {
  string message_id = 1;

  oneof payload {
    RegisterEndpoint register_endpoint = 10;
    HandshakeRequest handshake_request = 11;
    HandshakeResponse handshake_response = 12;
    Ping ping = 13;

    EndpointRegistered endpoint_registered = 20;
    IncomingHandshake incoming_handshake = 21;
    HandshakeResult handshake_result = 22;
    Pong pong = 23;
    PeerEndpointUpdate peer_endpoint_update = 24;
  }
}

message RegisterEndpoint {
  string device_id = 1;
  string auth_token = 2;
  Endpoint endpoint = 3;
}

message Endpoint {
  string public_addr = 1;
  NATType nat_type = 2;
  int64 timestamp = 3;
}

enum NATType {
  NAT_TYPE_UNKNOWN = 0;
  NAT_TYPE_NONE = 1;
  NAT_TYPE_FULL_CONE = 2;
  NAT_TYPE_RESTRICTED = 3;
  NAT_TYPE_PORT_RESTRICTED = 4;
  NAT_TYPE_SYMMETRIC = 5;
}

message EndpointRegistered {
  bool success = 1;
  string error = 2;
}

message HandshakeRequest {
  string target_vip = 1;
  string wg_public_key = 2;
  Endpoint my_endpoint = 3;
}

message IncomingHandshake {
  string from_device_id = 1;
  string from_vip = 2;
  string wg_public_key = 3;
  Endpoint peer_endpoint = 4;
}

message HandshakeResponse {
  string target_device_id = 1;
  bool accept = 2;
  string wg_public_key = 3;
  Endpoint my_endpoint = 4;
}

message HandshakeResult {
  string peer_device_id = 1;
  string peer_vip = 2;
  bool success = 3;
  string wg_public_key = 4;
  Endpoint peer_endpoint = 5;
  string error = 6;
}

message PeerEndpointUpdate {
  string peer_device_id = 1;
  string peer_vip = 2;
  Endpoint new_endpoint = 3;
}

message Ping {
  int64 timestamp = 1;
}

message Pong {
  int64 timestamp = 1;
}
```

## 11. 错误处理

### 11.1 服务端连接失败

| 场景 | 处理策略 |
|-----|---------|
| Management 连接失败 | 指数退避重试（1s → 2s → 4s → ... → 60s 上限） |
| Signal 连接断开 | 立即重连，退避重试，期间已建立的 P2P 隧道继续工作 |
| 认证失败 | 记录错误日志，不重试（可能是密钥被吊销） |
| 配置拉取失败 | 使用本地缓存配置继续运行 |

### 11.2 NAT 穿透失败

- 尝试 P2P 打洞，超时（3秒）后切换到 Relay
- 记录到本地，下次直接走 Relay
- 后台任务每 5 分钟重新尝试 P2P，成功则切回直连

### 11.3 WireGuard 引擎错误

| 场景 | 处理策略 |
|-----|---------|
| 内核模块不可用 | 自动降级到 wireguard-go |
| wireguard-go 启动失败 | 记录错误，退出（致命错误） |
| Peer 添加失败 | 重试 3 次，失败则上报 Signal Server |
| 网卡创建失败 | 检查权限，提示需要 root/CAP_NET_ADMIN |

### 11.4 本地网络异常

| 场景 | 处理策略 |
|-----|---------|
| 无网络连接 | 进入等待状态，监听网络恢复事件 |
| DNS 解析失败 | 使用缓存的服务端 IP，或尝试备用地址 |
| 路由添加失败 | 检查冲突路由，尝试清理后重试 |

### 11.5 资源限制

| 资源 | 限制 | 超限处理 |
|-----|------|---------|
| 内存 | < 64MB | 限制 Peer 缓存数量，使用 LRU 淘汰 |
| CPU | 闲时 < 1% | 批量处理事件，避免频繁唤醒 |
| 文件描述符 | 预留 1024 | 启动时检查 ulimit |
| Peer 数量 | 单 Agent 最大 1000 | 超出时拒绝新连接，记录告警 |

## 12. 实现优先级

### Phase 1（P0 - 核心功能）

目标：1000 节点测试

- [ ] identity - 密钥对生成与加载
- [ ] wireguard - wireguard-go 集成（先不做内核模式）
- [ ] client - Management 客户端（注册 + 认证）
- [ ] client - Signal 客户端（双向流 + 握手）
- [ ] nat - 基础 STUN 探测
- [ ] route - 路由表管理

### Phase 2（P1 - 完整体验）

- [ ] wireguard - 内核模块支持（混合模式）
- [ ] network - IP 变化监听与漫游
- [ ] identity - 设备指纹采集
- [ ] config - 配置文件加载
- [ ] 状态持久化 - 重启快速恢复

### Phase 3（P2 - 生产就绪）

- [ ] Relay 降级 - P2P 失败自动切换
- [ ] ACL 支持 - 访问控制规则
- [ ] 监控指标 - Prometheus metrics
- [ ] 日志完善 - 结构化日志
- [ ] 自动更新 - Agent 自升级机制

## 13. 依赖库

```go
require (
    // WireGuard
    golang.zx2c4.com/wireguard
    golang.zx2c4.com/wireguard/wgctrl

    // 网络操作
    github.com/vishvananda/netlink
    github.com/pion/stun

    // gRPC
    google.golang.org/grpc
    google.golang.org/protobuf

    // 配置与日志
    gopkg.in/yaml.v3
    go.uber.org/zap

    // CLI
    github.com/spf13/cobra
)
```

## 14. 预估代码量

| 模块 | 预估行数 | 说明 |
|-----|---------|------|
| identity | ~300 | 密钥 + 指纹 |
| wireguard | ~800 | 引擎 + Peer 管理 |
| nat | ~400 | STUN 探测 |
| route | ~200 | 路由操作 |
| network | ~300 | 网络监听 |
| client | ~600 | gRPC 客户端 |
| config | ~200 | 配置加载 |
| agent (主逻辑) | ~500 | 生命周期管理 |
| **合计** | **~3300** | 不含 proto 生成代码 |
