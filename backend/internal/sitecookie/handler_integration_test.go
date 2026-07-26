package sitecookie

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

type handlerEnv struct {
	pool   *pgxpool.Pool
	engine http.Handler
	store  *Store
}

func newHandlerEnv(t *testing.T) handlerEnv {
	t.Helper()
	pool := databasetest.New(t)
	engine := apitest.NewEngine(t)
	store := NewStore(filepath.Join(t.TempDir(), "cookies", "bilibili.txt"))
	h := &Handler{Store: store, Jobs: job.NewService(pool)}
	h.RegisterRoutes(engine.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	return handlerEnv{pool: pool, engine: engine, store: store}
}

func jsonHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/json"}
}

func idemHeaders() map[string]string {
	return map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": "019b3e60-7c9a-7f4e-8a4b-2f4d6c8a0e11",
	}
}

func TestBilibiliCookieLifecycleAndContractShape(t *testing.T) {
	env := newHandlerEnv(t)
	contract := apitest.Load(t)

	// 初始：未配置。
	get := apitest.Do(t, env.engine, http.MethodGet, "/api/v1/system/bilibili-cookie", nil, "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET 应 200, got %d %s", get.Code, get.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, "/api/v1/system/bilibili-cookie", http.StatusOK, get.Body.Bytes())
	var st Status
	if err := json.Unmarshal(get.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Configured {
		t.Fatalf("初始应为 configured=false: %+v", st)
	}

	// 保存（Netscape 内容，含非 bilibili 条目）。
	put := apitest.Do(t, env.engine, http.MethodPut, "/api/v1/system/bilibili-cookie",
		jsonHeaders(), `{"content": `+strconvQuote(netscapeContent)+`}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT 应 200, got %d %s", put.Code, put.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPut, "/api/v1/system/bilibili-cookie", http.StatusOK, put.Body.Bytes())
	if strings.Contains(put.Body.String(), testSessdata) {
		t.Fatal("响应绝不包含完整 SESSDATA")
	}
	if err := json.Unmarshal(put.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Configured || st.CookieCount != 3 || st.MaskedSessdata == "" || st.Expired {
		t.Fatalf("保存后状态不符: %+v", st)
	}

	// 重复 PUT 相同内容（幂等）→ 仍 200。
	putAgain := apitest.Do(t, env.engine, http.MethodPut, "/api/v1/system/bilibili-cookie",
		jsonHeaders(), `{"content": `+strconvQuote(netscapeContent)+`}`)
	if putAgain.Code != http.StatusOK {
		t.Fatalf("重复 PUT 应 200, got %d", putAgain.Code)
	}

	// 删除 → 204 → GET 回到未配置。
	del := apitest.Do(t, env.engine, http.MethodDelete, "/api/v1/system/bilibili-cookie", nil, "")
	if del.Code != http.StatusNoContent {
		t.Fatalf("DELETE 应 204, got %d %s", del.Code, del.Body.String())
	}
	delAgain := apitest.Do(t, env.engine, http.MethodDelete, "/api/v1/system/bilibili-cookie", nil, "")
	if delAgain.Code != http.StatusNotFound {
		t.Fatalf("重复 DELETE 应 404, got %d", delAgain.Code)
	}
}

func TestBilibiliCookieSaveRejectsInvalidWithoutSecretEcho(t *testing.T) {
	env := newHandlerEnv(t)

	cases := map[string]string{
		"empty_json":     `{"content": "  "}`,
		"not_cookie":     `{"content": "hello world this is not a cookie"}`,
		"missing_sess":   `{"content": ".bilibili.com\tTRUE\t/\tTRUE\t1800601560\tbili_jct\tx\n"}`,
		"missing_field":  `{}`,
		"malformed_json": `{"content": `,
	}
	// missing_sess 需要真实制表符/换行（backtick 内 \t 只是字面两字符）。
	cases["missing_sess"] = `{"content": ` + strconvQuote(".bilibili.com\tTRUE\t/\tTRUE\t1800601560\tbili_jct\tx\n") + `}`
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := apitest.Do(t, env.engine, http.MethodPut, "/api/v1/system/bilibili-cookie",
				jsonHeaders(), body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("无效内容应 400, got %d %s", rec.Code, rec.Body.String())
			}
			apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
		})
	}
	// 无效保存不得创建文件。
	st, err := env.store.Status()
	if err != nil || st.Configured {
		t.Fatalf("无效保存不得写入: %+v %v", st, err)
	}
}

func TestBilibiliCookieTestCreatesJob(t *testing.T) {
	env := newHandlerEnv(t)

	create := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/bilibili-cookie/tests",
		idemHeaders(), "")
	if create.Code != http.StatusAccepted {
		t.Fatalf("POST tests 应 202, got %d %s", create.Code, create.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost, "/api/v1/system/bilibili-cookie/tests",
		http.StatusAccepted, create.Body.Bytes())
	var accepted job.Accepted
	if err := json.Unmarshal(create.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Job.Kind != job.KindBilibiliCookieTest {
		t.Fatalf("Job kind 不符: %s", accepted.Job.Kind)
	}

	// 相同 Idempotency-Key 重放 → 返回同一个 Job。
	replay := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/bilibili-cookie/tests",
		idemHeaders(), "")
	if replay.Code != http.StatusAccepted {
		t.Fatalf("重放应 202, got %d", replay.Code)
	}
	var replayed job.Accepted
	if err := json.Unmarshal(replay.Body.Bytes(), &replayed); err != nil {
		t.Fatal(err)
	}
	if replayed.Job.ID != accepted.Job.ID {
		t.Fatal("相同 Idempotency-Key 应返回同一个 Job")
	}
}

// strconvQuote 把任意内容安全嵌入 JSON 字符串字面量。
func strconvQuote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
