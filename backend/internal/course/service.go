package course

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// CourseDTO 对齐契约 Course Schema（additionalProperties:false，
// slug 为服务端内部字段，不出现在 DTO 中）。
type CourseDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	LessonCount int64  `json:"lesson_count"`
	Version     int64  `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// toDTO 映射领域视图为契约 DTO（description NULL→""，时间为 UTC RFC 3339）。
func toDTO(c CourseWithCount) CourseDTO {
	desc := ""
	if c.Description != nil {
		desc = *c.Description
	}
	return CourseDTO{
		ID:          c.ID.String(),
		Title:       c.Title,
		Description: desc,
		Archived:    c.Archived,
		LessonCount: c.LessonCount,
		Version:     c.Version,
		CreatedAt:   httpx.Time(c.CreatedAt),
		UpdatedAt:   httpx.Time(c.UpdatedAt),
	}
}

// Service 是课程业务编排：拥有事务边界（pool.Begin），写事务入口
// 先过 gate.AssertBusinessOpen。
type Service struct {
	pool *pgxpool.Pool
	repo Repository
}

// NewService 构造课程 Service。
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateInput 是创建课程的入参（字段校验已在 handler 完成）。
type CreateInput struct {
	Title       string
	Description *string
}

// UpdateInput 是 PATCH 入参：指针字段为 nil 表示不更新；
// Version 为客户端持有的当前版本（乐观锁，必填）。
type UpdateInput struct {
	Title       *string
	Description *string
	Archived    *bool
	Version     int64
}

// List 按 created_at ASC, id ASC 游标分页；返回截断到 f.Limit 的一页与
// hasMore。游标所需排序键保留在返回的领域行中（CreatedAt 为全精度）。
func (s *Service) List(ctx context.Context, f ListFilter) ([]CourseWithCount, bool, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	fetch := f
	fetch.Limit = f.Limit + 1 // 多取一行判定 has_more
	rows, err := s.repo.List(ctx, s.pool, fetch)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > f.Limit
	if hasMore {
		rows = rows[:f.Limit]
	}
	return rows, hasMore, nil
}

// Create 在事务中创建课程（slug 服务端生成，唯一冲突时换后缀重试）。
// complete 非 nil 时在业务写入之后、提交之前于同一事务调用
// （幂等完成记录与业务写入同事务提交，03 §6.5 规则 4）。
func (s *Service) Create(ctx context.Context, in CreateInput,
	complete func(ctx context.Context, tx database.Querier, dto CourseDTO) error) (CourseDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return CourseDTO{}, err
	}
	var c Course
	created := false
	for range 5 {
		title := strings.TrimSpace(in.Title)
		pending := title == ""
		if pending {
			title = "待 AI 命名"
		}
		c = Course{Slug: newSlug(title), Title: title, Description: in.Description, TitlePending: pending}
		err = s.repo.Create(ctx, tx, &c)
		if errors.Is(err, ErrSlugTaken) {
			continue
		}
		if err != nil {
			return CourseDTO{}, err
		}
		created = true
		break
	}
	if !created {
		return CourseDTO{}, errors.New("生成唯一课程 slug 失败")
	}
	dto := toDTO(CourseWithCount{Course: c})
	if complete != nil {
		if err := complete(ctx, tx, dto); err != nil {
			return CourseDTO{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CourseDTO{}, err
	}
	return dto, nil
}

// Get 读取课程详情；不存在返回 ErrNotFound。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (CourseDTO, error) {
	c, err := s.repo.GetWithCount(ctx, s.pool, id)
	if err != nil {
		return CourseDTO{}, err
	}
	return toDTO(*c), nil
}

// Update 乐观锁更新：in.Version 不匹配返回 ErrVersionConflict；
// 允许不带任何可更新字段（仅校验版本并触发 updated_at）。
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (CourseDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return CourseDTO{}, err
	}
	c, err := s.repo.GetWithCount(ctx, tx, id)
	if err != nil {
		return CourseDTO{}, err
	}
	if in.Title != nil {
		c.Title = strings.TrimSpace(*in.Title)
		c.TitlePending = false
	}
	if in.Description != nil {
		c.Description = in.Description
	}
	if in.Archived != nil {
		c.Archived = *in.Archived
	}
	c.Version = in.Version
	if err := s.repo.Update(ctx, tx, &c.Course); err != nil {
		return CourseDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CourseDTO{}, err
	}
	return toDTO(*c), nil
}

// Delete 物理删除课程及全部子数据；任一课时有活动 lesson_generation
// 任务时拒绝（409 resource_busy）。
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.DeleteMany(ctx, []uuid.UUID{id})
}

// DeleteMany 原子批量删除课程：单个事务内先对全部 id 做存在性与
// 活动 lesson_generation 任务检查，任一失败整体回滚（404/409
// resource_busy）；全部通过后在同一事务逐个级联删除。
func (s *Service) DeleteMany(ctx context.Context, ids []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return err
	}
	if err := s.repo.LockForDelete(ctx, tx, ids); err != nil {
		return err
	}
	for _, id := range ids {
		busy, err := s.repo.HasActiveGenerationJob(ctx, tx, id)
		if err != nil {
			return err
		}
		if busy {
			return httpx.NewProblemError(409, "resource_busy", "资源繁忙", "存在进行中的任务，不能删除。")
		}
	}
	for _, id := range ids {
		if err := s.repo.DeleteCascade(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// newSlug 由标题生成 slug：ASCII 小写化（非 ASCII 丢弃，空白/标点折叠为
// 连字符，空则 "course"）+ "-" + uuid 前 8 位。slug 列 VARCHAR(220)，
// base 截断到 210 字节（保留字符均为单字节，无截断符文风险）。
func newSlug(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	prevDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r > 127:
			// 非 ASCII 字符丢弃
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		base = "course"
	}
	if len(base) > 210 {
		base = strings.TrimRight(base[:210], "-")
	}
	return base + "-" + uuid.NewString()[:8]
}
