// Package idempotency 实现通用 HTTP Idempotency-Key 语义（03 §6.5、04 §2.3）：
//
//   - 相同 scope/key + 相同请求 hash → 回放首次状态码与响应；
//   - 相同 scope/key + 不同请求 hash → 409 idempotency_conflict；
//   - 首次请求仍在处理 → 409 idempotency_in_progress + Retry-After；
//   - 普通业务写入与幂等完成记录在同一事务提交；
//   - 文件两阶段发布保留 in_progress，由 Recovery 完成或清理，
//     绝不生成第二份资源。
package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

// 记录状态。
const (
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
)

// Outcome 是 Begin 的判定结果。
type Outcome string

const (
	// OutcomeFresh：首次请求，调用方执行业务并在同一事务 Complete。
	OutcomeFresh Outcome = "fresh"
	// OutcomeReplay：相同请求已完成，调用方直接返回保存的响应。
	OutcomeReplay Outcome = "replay"
	// OutcomeConflict：相同 Key 不同请求体，映射 409 idempotency_conflict。
	OutcomeConflict Outcome = "conflict"
	// OutcomeInProgress：相同请求仍在处理，映射 409 idempotency_in_progress。
	OutcomeInProgress Outcome = "in_progress"
)

// Record 是 idempotency_records 表的领域表示。
type Record struct {
	ID              uuid.UUID
	Scope           string
	Key             string
	Method          string
	RouteTemplate   string
	RequestHash     string
	State           string
	ResponseStatus  *int
	ResponseBody    []byte
	ResponseHeaders []byte
	ResourceType    *string
	ResourceID      *uuid.UUID
	JobID           *uuid.UUID
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

// BeginInput 描述一次幂等请求。
type BeginInput struct {
	Scope         string // 通常为路由级资源范围，如 "course:create"
	Key           string // Idempotency-Key 头（前端为一次用户意图生成的 UUID）
	Method        string
	RouteTemplate string
	RequestHash   string // 方法+路由+规范化请求元数据+上传文件 SHA-256
	TTL           time.Duration
}

// Service 提供幂等判定。pool 用于 Begin 的独立短事务。
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Begin 判定一次请求。并发同一 Key 通过 advisory xact lock 串行化，
// 保证只有一个调用方得到 OutcomeFresh（03 §6.5 规则 1-3）。
func (s *Service) Begin(ctx context.Context, in BeginInput) (*Record, Outcome, error) {
	if in.Scope == "" || in.Key == "" || in.RequestHash == "" {
		return nil, "", errors.New("idempotency: scope/key/request_hash 不能为空")
	}
	ttl := in.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// 串行化同一 (scope,key) 的并发 Begin：后到者等待先到者提交后再判定。
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, in.Scope, in.Key); err != nil {
		return nil, "", err
	}
	rec, err := s.get(ctx, tx, in.Scope, in.Key)
	if err != nil {
		return nil, "", err
	}
	if rec != nil {
		outcome := OutcomeInProgress
		if rec.State == StateCompleted {
			if rec.RequestHash == in.RequestHash {
				outcome = OutcomeReplay
			} else {
				outcome = OutcomeConflict
			}
		} else if rec.RequestHash != in.RequestHash {
			// in_progress 但请求体不同：同样视为冲突（Key 被复用）。
			outcome = OutcomeConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, "", err
		}
		return rec, outcome, nil
	}
	rec = &Record{
		ID: uuid.New(), Scope: in.Scope, Key: in.Key, Method: in.Method,
		RouteTemplate: in.RouteTemplate, RequestHash: in.RequestHash,
		State: StateInProgress,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO idempotency_records (id, scope, key, method, route_template,
			request_hash, state, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,'in_progress', now() + $7::interval)`,
		rec.ID, rec.Scope, rec.Key, rec.Method, rec.RouteTemplate,
		rec.RequestHash, ttl.String()); err != nil {
		return nil, "", fmt.Errorf("idempotency: 创建记录失败: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return rec, OutcomeFresh, nil
}

// Complete 在业务写入的同一事务中完成记录（03 §6.5 规则 4）。
// response_body 只保存创建/任务响应所需的非敏感 JSON；headers 为需随回放
// 返回的响应头（如 Location），为 nil 时不保存。
func (s *Service) Complete(ctx context.Context, tx database.Querier, id uuid.UUID,
	status int, body, headers []byte, resourceType *string, resourceID, jobID *uuid.UUID) error {
	if body == nil {
		body = []byte(`{}`)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE idempotency_records
		SET state = 'completed', response_status = $2, response_body = $3,
		    response_headers = $4, resource_type = $5, resource_id = $6, job_id = $7
		WHERE id = $1 AND state = 'in_progress'`,
		id, status, body, headers, resourceType, resourceID, jobID)
	if err != nil {
		return fmt.Errorf("idempotency: 完成记录失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("idempotency: 记录不存在或已完成")
	}
	return nil
}

// Abandon 在业务事务提交前失败时删除 in_progress 记录
// （03 §6.5 规则 5：可安全重试的事务前失败不留 completed 记录）。
func (s *Service) Abandon(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM idempotency_records WHERE id = $1 AND state = 'in_progress'`, id)
	return err
}

// Get 读取记录；不存在返回 (nil, nil)。
func (s *Service) Get(ctx context.Context, scope, key string) (*Record, error) {
	return s.get(ctx, s.pool, scope, key)
}

func (s *Service) get(ctx context.Context, q database.Querier, scope, key string) (*Record, error) {
	var r Record
	err := q.QueryRow(ctx, `
		SELECT id, scope, key, method, route_template, request_hash, state,
		       response_status, response_body, response_headers, resource_type, resource_id, job_id,
		       created_at, expires_at
		FROM idempotency_records WHERE scope = $1 AND key = $2`, scope, key).Scan(
		&r.ID, &r.Scope, &r.Key, &r.Method, &r.RouteTemplate, &r.RequestHash, &r.State,
		&r.ResponseStatus, &r.ResponseBody, &r.ResponseHeaders, &r.ResourceType, &r.ResourceID, &r.JobID,
		&r.CreatedAt, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CleanupExpired 删除已过保留期的记录（默认至少保留 7 天，由 expires_at 决定）。
func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM idempotency_records WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RecoverInProgress 文件发布恢复（03 §6.5 规则 4 后半）：
// 文件两阶段发布保留 in_progress 记录；发布若从未完成（temporary 对象
// 已被文件 Recovery 隔离/删除，且业务事务从未提交），删除该 in_progress
// 记录使客户端可用原 Key 安全重试——不会生成第二份资源，因为资源创建
// 事务从未提交。grace 是在途请求宽限期（必须远大于请求超时）。
func (s *Service) RecoverInProgress(ctx context.Context, grace time.Duration) (int64, error) {
	if grace <= 0 {
		grace = time.Hour
	}
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM idempotency_records r
		WHERE r.state = 'in_progress'
		  AND r.created_at < now() - $1::interval
		  AND NOT EXISTS (
		    SELECT 1 FROM file_objects f
		    WHERE f.temporary_owner_type = 'idempotency'
		      AND f.temporary_owner_id = r.id
		  )`, grace.String())
	if err != nil {
		if isUndefinedTable(err) {
			return 0, nil
		}
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}
