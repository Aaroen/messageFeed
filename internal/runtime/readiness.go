package runtime

import "time"

const (
	// ReadinessReady 表示当前检查项或整体服务已经具备接收流量的条件。
	ReadinessReady = "ready"

	// ReadinessNotReady 表示当前检查项或整体服务暂不具备接收流量的条件。
	ReadinessNotReady = "not_ready"
)

// ReadinessCheck 表示单个就绪检查项。
// 第一阶段只有 process 检查；后续 PostgreSQL、迁移状态、任务锁等依赖可以继续追加。
type ReadinessCheck struct {
	// Name 是检查项名称，例如 process、postgres 或 migrations。
	Name string `json:"name"`

	// Status 是检查项状态，当前允许 ready 和 not_ready。
	Status string `json:"status"`

	// Message 是面向排障的简短说明。
	// 该字段不应包含密钥、DSN、Webhook 等敏感配置。
	Message string `json:"message,omitempty"`
}

// ReadinessReport 表示 /readyz 的响应主体。
// 它同时提供整体状态和细分检查项，便于后续在不改变 API 形态的情况下扩展依赖检查。
type ReadinessReport struct {
	// Status 是整体就绪状态。
	// 只要存在一个 not_ready 检查项，整体状态就应为 not_ready。
	Status string `json:"status"`

	// Checks 保存所有参与就绪判断的检查项。
	Checks []ReadinessCheck `json:"checks"`

	// CheckedAt 是本次就绪检查生成时间。
	// 使用 UTC 时间便于后续与多节点日志和指标进行比较。
	CheckedAt time.Time `json:"checked_at"`
}

// NewProcessReadinessReport 构建第一阶段的进程级就绪报告。
// 当前阶段尚未接入数据库，因此只要 API 进程可以执行到该函数，即认为进程级检查通过。
func NewProcessReadinessReport(checkedAt time.Time) ReadinessReport {
	return NewReadinessReport([]ReadinessCheck{
		{
			Name:    "process",
			Status:  ReadinessReady,
			Message: "api process is running",
		},
	}, checkedAt)
}

// NewReadinessReport 根据检查项聚合整体就绪状态。
// 该函数会复制 checks 切片，避免调用方后续修改输入切片影响已生成的响应。
func NewReadinessReport(checks []ReadinessCheck, checkedAt time.Time) ReadinessReport {
	status := ReadinessReady
	for _, check := range checks {
		if check.Status != ReadinessReady {
			status = ReadinessNotReady
			break
		}
	}

	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	return ReadinessReport{
		Status:    status,
		Checks:    append([]ReadinessCheck(nil), checks...),
		CheckedAt: checkedAt.UTC(),
	}
}

// Ready 返回整体就绪状态是否为 ready。
// handler 层可使用该方法决定返回 HTTP 200 还是 HTTP 503。
func (report ReadinessReport) Ready() bool {
	return report.Status == ReadinessReady
}
