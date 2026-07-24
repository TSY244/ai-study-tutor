package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/maintenance"
)

const (
	statusProbeTimeout = 2 * time.Second
	workerHeartbeatTTL = 30 * time.Second
)

// ComponentStatus is the common OpenAPI component shape. The optional fields
// are component-specific, non-sensitive diagnostics allowed by the contract.
type ComponentStatus struct {
	Status           string  `json:"status"`
	Message          *string `json:"message"`
	MigrationVersion *string `json:"migration_version,omitempty"`
	LastHeartbeatAt  *string `json:"last_heartbeat_at,omitempty"`
	ActiveJobs       *int64  `json:"active_jobs,omitempty"`
	QueuedJobs       *int64  `json:"queued_jobs,omitempty"`
	Model            *string `json:"model,omitempty"`
	LastCheckedAt    *string `json:"last_checked_at,omitempty"`
	FreeBytes        *uint64 `json:"free_bytes,omitempty"`
}

type MaintenanceStatus struct {
	Enabled bool    `json:"enabled"`
	Reason  *string `json:"reason"`
}

// StatusResponse is the published, non-sensitive system status response.
type StatusResponse struct {
	Status      string            `json:"status"`
	Version     string            `json:"version"`
	Maintenance MaintenanceStatus `json:"maintenance"`
	Database    ComponentStatus   `json:"database"`
	Worker      ComponentStatus   `json:"worker"`
	Claude      ComponentStatus   `json:"claude"`
	Storage     ComponentStatus   `json:"storage"`
}

// StatusService aggregates dependencies visible from the API process. It does
// not read the Worker-only Anthropic secret.
type StatusService struct {
	pool    *pgxpool.Pool
	dataDir string
	version string
}

func NewStatusService(pool *pgxpool.Pool, dataDir, version string) *StatusService {
	return &StatusService{pool: pool, dataDir: dataDir, version: version}
}

// Get returns a diagnostic snapshot even when one dependency is unavailable.
// A status endpoint must explain degradation instead of turning a dependency
// failure into an opaque HTTP 500.
func (s *StatusService) Get(ctx context.Context) StatusResponse {
	result := StatusResponse{
		Status:  "degraded",
		Version: s.version,
		Maintenance: MaintenanceStatus{
			Enabled: false,
			Reason:  nil,
		},
		Database: ComponentStatus{Status: "unknown"},
		Worker:   ComponentStatus{Status: "unknown"},
		Claude:   ComponentStatus{Status: "unknown"},
		Storage:  storageStatus(s.dataDir),
	}

	result.Database = s.databaseStatus(ctx)
	if result.Database.Status == "ok" {
		var maintenanceOK bool
		result.Maintenance, maintenanceOK = s.maintenanceStatus(ctx)
		if !maintenanceOK {
			result.Database.Status = "degraded"
			result.Database.Message = message("无法读取维护状态。")
		}
		worker, capability := s.workerStatus(ctx)
		result.Worker = worker
		result.Claude = s.claudeStatus(ctx, capability)
	} else {
		result.Worker = ComponentStatus{Status: "unknown", Message: message("数据库不可用，无法读取 Worker 状态。")}
		result.Claude = ComponentStatus{Status: "unknown", Message: message("数据库不可用，无法读取 Claude 状态。")}
	}

	result.Status = overallStatus(result)
	return result
}

func (s *StatusService) databaseStatus(ctx context.Context) ComponentStatus {
	if s.pool == nil {
		return ComponentStatus{Status: "unavailable", Message: message("数据库连接池未初始化。")}
	}
	probeCtx, cancel := context.WithTimeout(ctx, statusProbeTimeout)
	defer cancel()
	if err := s.pool.Ping(probeCtx); err != nil {
		return ComponentStatus{Status: "unavailable", Message: message("数据库暂时不可用。")}
	}
	var version string
	err := s.pool.QueryRow(probeCtx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return ComponentStatus{Status: "degraded", Message: message("数据库尚未应用迁移。")}
	}
	if err != nil {
		return ComponentStatus{Status: "degraded", Message: message("无法读取数据库迁移状态。")}
	}
	return ComponentStatus{Status: "ok", MigrationVersion: &version}
}

func (s *StatusService) maintenanceStatus(ctx context.Context) (MaintenanceStatus, bool) {
	state, err := (maintenance.Repository{}).Get(ctx, s.pool)
	if err != nil {
		return MaintenanceStatus{Enabled: false, Reason: message("无法读取维护状态。")}, false
	}
	return MaintenanceStatus{Enabled: state.MaintenanceMode, Reason: state.MaintenanceReason}, true
}

type workerCapability struct {
	claudeStatus  string
	claudeMessage *string
	claudeModel   *string
	fresh         bool
}

func (s *StatusService) workerStatus(ctx context.Context) (ComponentStatus, workerCapability) {
	var activeJobs, queuedJobs int64
	if err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status IN ('running', 'validating', 'cancel_requested') AND lease_expires_at > now()),
			count(*) FILTER (WHERE status = 'queued')
		FROM jobs`).Scan(&activeJobs, &queuedJobs); err != nil {
		return ComponentStatus{Status: "unknown", Message: message("无法读取 Worker 队列状态。")}, workerCapability{}
	}

	var (
		workerID, claudeStatus     string
		claudeMessage, claudeModel *string
		lastHeartbeat              time.Time
		fresh                      bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT worker_id, claude_status, claude_message, claude_model, last_heartbeat_at,
		       last_heartbeat_at > now() - $1::interval
		FROM worker_heartbeats
		ORDER BY last_heartbeat_at DESC
		LIMIT 1`, workerHeartbeatTTL.String()).Scan(
		&workerID, &claudeStatus, &claudeMessage, &claudeModel, &lastHeartbeat, &fresh)
	if errors.Is(err, pgx.ErrNoRows) {
		component := ComponentStatus{
			Status:     "unknown",
			Message:    message("尚未收到 Worker 心跳。"),
			ActiveJobs: &activeJobs,
			QueuedJobs: &queuedJobs,
		}
		if queuedJobs > 0 {
			component.Status = "unavailable"
			component.Message = message(fmt.Sprintf("尚未收到 Worker 心跳，当前有 %d 个排队任务。", queuedJobs))
		}
		return component, workerCapability{}
	}
	if err != nil {
		return ComponentStatus{Status: "unknown", Message: message("无法读取 Worker 心跳。")}, workerCapability{}
	}
	_ = workerID
	heartbeat := lastHeartbeat.UTC().Format(time.RFC3339)
	component := ComponentStatus{
		Status:          "ok",
		LastHeartbeatAt: &heartbeat,
		ActiveJobs:      &activeJobs,
		QueuedJobs:      &queuedJobs,
	}
	if !fresh {
		component.Status = "unavailable"
		component.Message = message("Worker 心跳已过期。")
	}
	return component, workerCapability{
		claudeStatus:  claudeStatus,
		claudeMessage: claudeMessage,
		claudeModel:   claudeModel,
		fresh:         fresh,
	}
}

func (s *StatusService) claudeStatus(ctx context.Context, capability workerCapability) ComponentStatus {
	if !capability.fresh {
		return ComponentStatus{Status: "unknown", Message: message("Worker 心跳不可用，无法确认当前 Claude 能力。")}
	}
	if capability.claudeStatus == "unavailable" || capability.claudeStatus == "degraded" {
		return ComponentStatus{
			Status:  capability.claudeStatus,
			Message: capability.claudeMessage,
			Model:   capability.claudeModel,
		}
	}

	var (
		status, model string
		errorType     *string
		checkedAt     time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT status, model, error_type, created_at
		FROM ai_runs
		ORDER BY created_at DESC
		LIMIT 1`).Scan(&status, &model, &errorType, &checkedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ComponentStatus{
			Status:  "unknown",
			Message: capability.claudeMessage,
			Model:   capability.claudeModel,
		}
	}
	if err != nil {
		return ComponentStatus{Status: "unknown", Message: message("无法读取最近一次 Claude 调用状态。")}
	}
	checked := checkedAt.UTC().Format(time.RFC3339)
	component := ComponentStatus{Status: "ok", Model: &model, LastCheckedAt: &checked}
	if status != "success" {
		component.Status = "unavailable"
		if errorType != nil && *errorType != "" {
			component.Message = message(fmt.Sprintf("最近一次 Claude 调用失败（%s）。", *errorType))
		} else {
			component.Message = message("最近一次 Claude 调用失败。")
		}
	}
	return component
}

func storageStatus(dataDir string) ComponentStatus {
	if dataDir == "" {
		return ComponentStatus{Status: "unknown", Message: message("数据目录未配置。")}
	}
	if _, err := os.Stat(dataDir); err != nil {
		return ComponentStatus{Status: "unavailable", Message: message("数据目录不可访问。")}
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		return ComponentStatus{Status: "unknown", Message: message("无法读取数据目录容量。")}
	}
	freeBytes := uint64(stat.Bavail) * uint64(stat.Bsize)
	return ComponentStatus{Status: "ok", FreeBytes: &freeBytes}
}

func overallStatus(status StatusResponse) string {
	if status.Database.Status == "unavailable" {
		return "unavailable"
	}
	if status.Maintenance.Enabled {
		return "degraded"
	}
	for _, component := range []ComponentStatus{status.Database, status.Worker, status.Claude, status.Storage} {
		if component.Status != "ok" {
			return "degraded"
		}
	}
	return "ok"
}

func message(value string) *string { return &value }
