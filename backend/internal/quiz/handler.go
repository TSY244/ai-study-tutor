package quiz

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// QueryHandler 只做 HTTP 绑定与响应映射；业务在 QueryService。
type QueryHandler struct {
	svc *QueryService
}

func NewQueryHandler(svc *QueryService) *QueryHandler {
	return &QueryHandler{svc: svc}
}

// RegisterRoutes 注册试卷查询端点（只读，不要求幂等键）。
func (h *QueryHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/lessons/:lesson_id/quizzes", h.listQuizzes)
	rg.GET("/quizzes/:quiz_id", h.getQuiz)
}

// GET /api/v1/lessons/:lesson_id/quizzes —— 课时试卷列表（游标分页）。
func (h *QueryHandler) listQuizzes(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "limit 必须是 1-100 的整数。")
		return
	}
	var afterCreatedAt *time.Time
	var afterID *uuid.UUID
	if raw := c.Query("cursor"); raw != "" {
		var cur map[string]any
		if !httpx.DecodeCursor(raw, &cur) {
			httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
				"请求无效", "cursor 无法解析。")
			return
		}
		ts, ok1 := cur["created_at"].(string)
		id, ok2 := cur["id"].(string)
		t, err1 := time.Parse(time.RFC3339, ts)
		u, err2 := uuid.Parse(id)
		if !ok1 || !ok2 || err1 != nil || err2 != nil {
			httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
				"请求无效", "cursor 无法解析。")
			return
		}
		afterCreatedAt = &t
		afterID = &u
	}
	rows, err := h.svc.ListQuizzes(c.Request.Context(), lessonID, afterCreatedAt, afterID, limit)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	items := make([]QuizSummaryDTO, 0, limit)
	for _, r := range rows {
		if len(items) == limit {
			break
		}
		items = append(items, ToQuizSummaryDTO(r))
	}
	page := httpx.PageInfo{HasMore: len(rows) > limit}
	if page.HasMore && len(items) > 0 {
		last := rows[len(items)-1]
		next := httpx.EncodeCursor(map[string]any{
			"created_at": last.CreatedAt.UTC().Format(time.RFC3339Nano),
			"id":         last.ID.String(),
		})
		page.NextCursor = &next
	}
	c.JSON(http.StatusOK, httpx.Collection[QuizSummaryDTO]{Items: items, PageInfo: page})
}

// GET /api/v1/quizzes/:quiz_id —— 试卷及学生可见题目快照。
func (h *QueryHandler) getQuiz(c *gin.Context) {
	quizID, ok := pathUUID(c, "quiz_id")
	if !ok {
		return
	}
	z, questions, err := h.svc.GetQuiz(c.Request.Context(), quizID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ToQuizDTO(z, questions))
}
