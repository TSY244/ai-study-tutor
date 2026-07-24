// Package api_test 是路由装配的冒烟测试：验证全部模块注册到同一引擎
// 时无 Gin 路由冲突（冲突会在注册时 panic），并抽查端到端通路。
package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/api"
	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

func TestRegisterRoutesSmoke(t *testing.T) {
	pool := databasetest.New(t)
	files, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	engine := apitest.NewEngine(t)
	// 全模块注册：任何参数名冲突/重复路由都会在此 panic。
	api.RegisterRoutes(engine, api.Deps{Pool: pool, Files: files})

	// GET /api/v1/courses 端到端通路（中间件 + handler + repository）。
	rec := apitest.Do(t, engine, http.MethodGet, "/api/v1/courses", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodGet, "/api/v1/courses",
		http.StatusOK, rec.Body.Bytes())

	// 未注册路径 → 404；已注册路径缺 Idempotency-Key → 400。
	rec = apitest.Do(t, engine, http.MethodGet, "/api/v1/nope", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("未知路径 status=%d", rec.Code)
	}
	rec = apitest.Do(t, engine, http.MethodPost, "/api/v1/courses",
		map[string]string{"Content-Type": "application/json"}, `{"title":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺幂等键 status=%d body=%s", rec.Code, rec.Body.String())
	}
}
