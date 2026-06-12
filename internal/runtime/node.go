package runtime

import "time"

// NodeInfo 描述当前 API 节点的运行时身份与访问信息。
// 该结构用于 /api/runtime/node 响应，也为后续多节点部署、健康入口和调度锁
// 提供统一的节点元数据。
type NodeInfo struct {
	// NodeID 是当前节点标识。
	// 第一阶段默认使用 local-dev，后续多节点部署时应由 APP_NODE_ID 显式指定。
	NodeID string `json:"node_id"`

	// DeploymentMode 表示部署拓扑，例如 single_node 或 cluster。
	// 该字段不决定监听范围，监听地址仍由 BIND_ADDR 控制。
	DeploymentMode string `json:"deployment_mode"`

	// PublicBaseURL 是用户或外部入口访问当前服务时使用的公开基址。
	// 本地、局域网、Tailscale 和后续 Cloudflare 入口都应通过该字段表达。
	PublicBaseURL string `json:"public_base_url"`

	// BindAddr 是 HTTP 服务的实际监听地址。
	// 该字段用于排查服务是否只绑定本机、绑定局域网地址或绑定 Tailscale 地址。
	BindAddr string `json:"bind_addr"`

	// TrustedProxyCIDRs 保存可信代理网段。
	// 第一阶段可以为空；后续接入反向代理或 Cloudflare 入口时用于识别可信来源。
	TrustedProxyCIDRs []string `json:"trusted_proxy_cidrs"`

	// StartedAt 是当前进程启动并构建节点信息的时间。
	// 使用 UTC 时间便于后续多节点日志、指标和审计记录进行统一比较。
	StartedAt time.Time `json:"started_at"`
}

// NodeOptions 是构建 NodeInfo 所需的输入参数。
// 入口层负责从配置模块读取这些值，再传入 runtime 模块构建稳定的节点快照。
type NodeOptions struct {
	// NodeID 对应配置中的 APP_NODE_ID。
	NodeID string

	// DeploymentMode 对应配置中的 DEPLOYMENT_MODE。
	DeploymentMode string

	// PublicBaseURL 对应配置中的 PUBLIC_BASE_URL。
	PublicBaseURL string

	// BindAddr 对应配置中的 BIND_ADDR。
	BindAddr string

	// TrustedProxyCIDRs 对应配置中的 TRUSTED_PROXY_CIDRS。
	TrustedProxyCIDRs []string

	// StartedAt 允许入口层传入统一启动时间；为空时由 runtime 模块自动生成。
	StartedAt time.Time
}

// NewNodeInfo 根据输入参数创建节点信息快照。
// 该函数会复制 TrustedProxyCIDRs，避免调用方后续修改切片导致响应内容被间接改变。
func NewNodeInfo(options NodeOptions) NodeInfo {
	startedAt := options.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	return NodeInfo{
		NodeID:            options.NodeID,
		DeploymentMode:    options.DeploymentMode,
		PublicBaseURL:     options.PublicBaseURL,
		BindAddr:          options.BindAddr,
		TrustedProxyCIDRs: append([]string(nil), options.TrustedProxyCIDRs...),
		StartedAt:         startedAt.UTC(),
	}
}
