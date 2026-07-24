package filestore_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/filestore"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

func newStore(t *testing.T, pool *pgxpool.Pool) *filestore.Store {
	t.Helper()
	st, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// stageFile 在 Owner 的 tmp 目录写入内容并返回绝对路径与 SHA-256。
func stageFile(t *testing.T, st *filestore.Store, ownerID uuid.UUID, content []byte) (string, string) {
	t.Helper()
	dir := st.TmpDirFor(ownerID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "staged.bin")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	return p, hex.EncodeToString(sum[:])
}

// seedCourseLesson 创建 material 外键所需的课程/课时。
func seedCourseLesson(t *testing.T, pool *pgxpool.Pool) (courseID, lessonID uuid.UUID) {
	t.Helper()
	courseID, lessonID = uuid.New(), uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO courses (id, slug, title) VALUES ($1,$2,'测试课程')`, courseID, "c-"+uuid.NewString()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'课时')`, lessonID, courseID); err != nil {
		t.Fatal(err)
	}
	return courseID, lessonID
}

// seedJob 创建一个指定状态的 Job（filestore Owner 证明用）。
func seedJob(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO jobs (id, kind, aggregate_type, aggregate_id, status)
		VALUES ($1,'lesson_import','lesson',$2,$3)`, id, uuid.New(), status); err != nil {
		t.Fatal(err)
	}
	return id
}

// 两阶段发布 happy path：temporary -> hash -> atomic rename -> ready + link 同事务。
func TestPublishHappyPathWithLink(t *testing.T) {
	pool := databasetest.New(t)
	st := newStore(t, pool)
	_, lessonID := seedCourseLesson(t, pool)
	jobID := seedJob(t, pool, "running")

	content := []byte("# 课程讲义\n确定性内容")
	staged, wantSHA := stageFile(t, st, jobID, content)

	var linkedMaterial uuid.UUID
	obj, err := st.Publish(context.Background(), filestore.PublishInput{
		OwnerType: filestore.OwnerTypeJob, OwnerID: jobID,
		StagedPath: staged, OriginalName: "notes.md", MediaType: "text/markdown",
	}, func(ctx context.Context, tx pgx.Tx, objectID uuid.UUID) error {
		linkedMaterial = uuid.New()
		// 业务实体引用必须在 tx2 内且对象已 ready。
		if err := st.AssertReady(ctx, tx, objectID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO materials (id, lesson_id, kind, file_object_id, status)
			VALUES ($1,$2,'upload',$3,'ready')`, linkedMaterial, lessonID, objectID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if obj.State != filestore.StateReady || obj.SHA256 != wantSHA || obj.SizeBytes != int64(len(content)) {
		t.Fatalf("发布结果不正确: %+v", obj)
	}
	// 文件在最终路径，tmp 已清空。
	finalAbs, err := st.AbsPath(obj.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(finalAbs); err != nil || string(got) != string(content) {
		t.Fatalf("最终文件内容不符: %v", err)
	}
	if _, err := os.Stat(st.TmpDirFor(jobID)); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(st.TmpDirFor(jobID))
		if len(entries) != 0 {
			t.Fatalf("tmp 目录应已排空: %v", entries)
		}
	}
	// 数据库行与业务引用一致。
	got, err := st.GetByID(context.Background(), pool, obj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != filestore.StateReady || got.TemporaryOwnerID != nil {
		t.Fatalf("ready 行仍残留 Owner: %+v", got)
	}
	var refCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM materials WHERE file_object_id = $1`, obj.ID).Scan(&refCount); err != nil {
		t.Fatal(err)
	}
	if refCount != 1 {
		t.Fatalf("material 引用数 = %d，期望 1", refCount)
	}
}

// 路径逃逸防护：暂存文件必须在数据目录 tmp/ 内。
func TestPublishRejectsPathEscape(t *testing.T) {
	pool := databasetest.New(t)
	st := newStore(t, pool)
	outside := filepath.Join(t.TempDir(), "evil.bin")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := st.Publish(context.Background(), filestore.PublishInput{
		OwnerType: filestore.OwnerTypeJob, OwnerID: uuid.New(), StagedPath: outside,
	}, nil)
	if !errors.Is(err, filestore.ErrPathEscape) {
		t.Fatalf("应拒绝路径逃逸，得到 %v", err)
	}
	if _, err := st.AbsPath("../../etc/passwd"); !errors.Is(err, filestore.ErrPathEscape) {
		t.Fatalf("AbsPath 应拒绝逃逸键，得到 %v", err)
	}
}

// insertExpiredTemporary 模拟 tx1 已提交、进程在 tx2 前崩溃的 temporary 行。
func insertExpiredTemporary(t *testing.T, pool *pgxpool.Pool, st *filestore.Store,
	ownerType string, ownerID uuid.UUID, sha string, size int64, expired bool) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	tmpKey := filepath.Join("tmp", ownerID.String(), id.String())
	expiry := "now() + interval '1 hour'"
	if expired {
		expiry = "now() - interval '1 second'"
	}
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`
		INSERT INTO file_objects (id, storage_key, size_bytes, sha256, state,
			temporary_owner_type, temporary_owner_id, temporary_expires_at)
		VALUES ($1,$2,$3,$4,'temporary',$5,$6,%s)`, expiry),
		id, tmpKey, size, sha, ownerType, ownerID); err != nil {
		t.Fatal(err)
	}
	return id, tmpKey
}

func recoverOnce(t *testing.T, pool *pgxpool.Pool, st *filestore.Store) filestore.RecoverResult {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	res, err := st.Recover(context.Background(), tx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return res
}

// 文件恢复矩阵：Owner 已提交 → ready；Owner 死亡 → quarantine；无文件 → deleted；Owner 存活 → 跳过。
func TestRecoverExpiredTemporaryMatrix(t *testing.T) {
	pool := databasetest.New(t)
	st := newStore(t, pool)
	ctx := context.Background()

	content := []byte("recovery matrix")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])

	// 1) Owner(job) 已 succeeded，文件已在最终路径 → 补全 ready。
	doneJob := seedJob(t, pool, "succeeded")
	idReady, tmpKeyReady := insertExpiredTemporary(t, pool, st, filestore.OwnerTypeJob, doneJob, sha, int64(len(content)), true)
	finalAbs := filepath.Join(st.DataDir(), "objects", sha[:2], idReady.String())
	if err := os.MkdirAll(filepath.Dir(finalAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(finalAbs, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// 2) Owner(job) failed 且文件还在 tmp → quarantine。
	deadJob := seedJob(t, pool, "failed")
	idQuar, tmpKeyQuar := insertExpiredTemporary(t, pool, st, filestore.OwnerTypeJob, deadJob, sha, int64(len(content)), true)
	tmpAbs := filepath.Join(st.DataDir(), tmpKeyQuar)
	if err := os.MkdirAll(filepath.Dir(tmpAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpAbs, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// 3) 文件不存在 → deleted。
	goneJob := seedJob(t, pool, "failed")
	idGone, _ := insertExpiredTemporary(t, pool, st, filestore.OwnerTypeJob, goneJob, sha, int64(len(content)), true)

	// 4) Owner(job) running 且租约有效 → 跳过。
	liveJob := seedJob(t, pool, "running")
	if _, err := pool.Exec(ctx,
		`UPDATE jobs SET lease_owner='w', lease_expires_at = now() + interval '1 minute' WHERE id=$1`, liveJob); err != nil {
		t.Fatal(err)
	}
	idLive, _ := insertExpiredTemporary(t, pool, st, filestore.OwnerTypeJob, liveJob, sha, int64(len(content)), true)

	// 5) 孤立最终对象（无数据库行）→ quarantine/orphan。
	orphanRel := filepath.Join("objects", "ab", uuid.NewString())
	orphanAbs := filepath.Join(st.DataDir(), orphanRel)
	if err := os.MkdirAll(filepath.Dir(orphanAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanAbs, []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := recoverOnce(t, pool, st)
	if res.Completed != 1 || res.Quarantined != 1 || res.Deleted != 1 || res.SkippedLive != 1 || res.Orphaned != 1 {
		t.Fatalf("恢复计数 = %+v，期望 {1 1 1 1 1}", res)
	}

	assertState := func(id uuid.UUID, want string) *filestore.Object {
		t.Helper()
		o, err := st.GetByID(ctx, pool, id)
		if err != nil {
			t.Fatal(err)
		}
		if o.State != want {
			t.Fatalf("对象 %s state = %s，期望 %s", id, o.State, want)
		}
		return o
	}
	o := assertState(idReady, filestore.StateReady)
	if _, err := os.Stat(filepath.Join(st.DataDir(), o.StorageKey)); err != nil {
		t.Fatalf("ready 对象文件应存在: %v", err)
	}
	o = assertState(idQuar, filestore.StateQuarantined)
	if _, err := os.Stat(filepath.Join(st.DataDir(), o.StorageKey)); err != nil {
		t.Fatalf("隔离文件应存在于 quarantine: %v", err)
	}
	assertState(idGone, filestore.StateDeleted)
	assertState(idLive, filestore.StateTemporary)
	_ = tmpKeyReady
}

// 业务实体不得引用 temporary 对象。
func TestAssertReadyRejectsTemporary(t *testing.T) {
	pool := databasetest.New(t)
	st := newStore(t, pool)
	jobID := seedJob(t, pool, "running")
	id, _ := insertExpiredTemporary(t, pool, st, filestore.OwnerTypeJob, jobID,
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 10, false)
	if err := st.AssertReady(context.Background(), pool, id); !errors.Is(err, filestore.ErrNotReady) {
		t.Fatalf("temporary 对象应被拒绝引用，得到 %v", err)
	}
	if err := st.AssertReady(context.Background(), pool, uuid.New()); !errors.Is(err, filestore.ErrNotFound) {
		t.Fatalf("不存在对象应返回 ErrNotFound，得到 %v", err)
	}
}
