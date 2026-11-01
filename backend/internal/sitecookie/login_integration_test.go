package sitecookie

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
)

func TestQRLoginHandoffAndPrivateCredentials(t *testing.T) {
	e := newHandlerEnv(t)
	const route = "/api/v1/system/bilibili-cookie/logins"
	r := apitest.Do(t, e.engine, http.MethodPost, route, idemHeaders(), "")
	if r.Code != http.StatusAccepted {
		t.Fatalf("create login: %d %s", r.Code, r.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost, route, http.StatusAccepted, r.Body.Bytes())
	var accepted job.Accepted
	if err := json.Unmarshal(r.Body.Bytes(), &accepted); err != nil || accepted.Job.Kind != job.KindBilibiliLogin {
		t.Fatal("login must be a Worker job")
	}
	url := route + "/" + accepted.Job.ID.String() + "/poll"
	r = apitest.Do(t, e.engine, http.MethodPost, url, jsonHeaders(), "")
	if r.Code != 200 || !strings.Contains(r.Body.String(), "generating") {
		t.Fatalf("queued login: %d %s", r.Code, r.Body.String())
	}
	id := accepted.Job.ID
	if err := os.MkdirAll(e.sessions.Profile(id), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.sessions.Profile(id), "login_qrcode.png"), []byte("\x89PNG\r\n\x1a\nfixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Minute).UTC().Format(time.RFC3339)
	if err := e.sessions.Write(id, LoginStatus{State: "scanned", Message: "请在手机确认", ExpiresAt: &expires}); err != nil {
		t.Fatal(err)
	}
	const sess = "QR-LOGIN-SESSDATA-SECRET-SENTINEL"
	const csrf = "QR-LOGIN-CSRF-SECRET-SENTINEL"
	raw, _ := json.Marshal(map[string]string{"sessdata": sess, "bili_jct": csrf, "dedeuserid": "42"})
	if err := os.WriteFile(filepath.Join(e.sessions.Profile(id), "credential.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	r = apitest.Do(t, e.engine, http.MethodPost, url, jsonHeaders(), "")
	if r.Code != 200 || r.Header().Get("Cache-Control") != "no-store" || !strings.Contains(r.Body.String(), "data:image/png;base64,") {
		t.Fatalf("QR response: %d %s", r.Code, r.Body.String())
	}
	if strings.Contains(r.Body.String(), sess) || strings.Contains(r.Body.String(), csrf) {
		t.Fatal("QR polling leaked credentials")
	}
	if st, _ := e.store.Status(); st.Configured {
		t.Fatal("credentials must not be imported before job confirmation")
	}
	if err := e.sessions.Write(id, LoginStatus{State: "confirmed", Message: "已确认"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE jobs SET status='succeeded', stage='succeeded', progress=100, completed_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		r = apitest.Do(t, e.engine, http.MethodPost, url, jsonHeaders(), "")
		if r.Code != 200 || !strings.Contains(r.Body.String(), "logged_in") || strings.Contains(r.Body.String(), sess) || strings.Contains(r.Body.String(), csrf) {
			t.Fatalf("safe, repeatable credential import: %d %s", r.Code, r.Body.String())
		}
		apitest.Load(t).ValidateJSONResponse(t, http.MethodPost, route+"/{login_id}/poll", http.StatusOK, r.Body.Bytes())
	}
	f, err := e.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if entry, ok := f.Sessdata(); !ok || entry.Value != sess {
		t.Fatal("saved Cookie must be usable by video imports")
	}
	if _, err := os.Stat(e.sessions.Profile(id)); !os.IsNotExist(err) {
		t.Fatal("temporary credentials must be removed after import")
	}
}

func TestQRLoginExpirationAndSessionValidation(t *testing.T) {
	e := newHandlerEnv(t)
	id := uuid.New()
	expires := time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
	if err := e.sessions.Write(id, LoginStatus{State: "waiting_scan", Message: "扫码", ExpiresAt: &expires}); err != nil {
		t.Fatal(err)
	}
	st, err := e.sessions.Read(id)
	if err != nil || st.State != "expired" || st.QRImageDataURL != "" {
		t.Fatal("expired QR codes must not be returned")
	}
	r := apitest.Do(t, e.engine, http.MethodPost, "/api/v1/system/bilibili-cookie/logins/invalid/poll", jsonHeaders(), "")
	if r.Code != http.StatusBadRequest {
		t.Fatal("invalid session IDs must be rejected")
	}
}
