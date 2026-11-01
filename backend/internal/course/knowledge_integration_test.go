package course_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/google/uuid"
)

func TestKnowledgeCatalogueReplacementAndContract(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	ctx := context.Background()
	c := createCourse(t, engine, uuid.NewString(), `{"title":"算法"}`)
	courseID := uuid.MustParse(c["id"].(string))
	lessonID := uuid.New()
	jobID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO lessons(id,course_id,ordinal,title) VALUES($1,$2,1,'二分查找')`, lessonID, courseID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,kind,aggregate_type,aggregate_id,payload) VALUES($1,'quiz_generate','lesson',$2,'{}')`, jobID, lessonID); err != nil {
		t.Fatal(err)
	}
	repo := course.Repository{}
	p := course.KnowledgeDraft{Label: "有序性", Explanation: "数据有序", LearningObjective: "判断前提", SourceExcerpt: "有序", Importance: "core"}
	replace := func(points []course.KnowledgeDraft) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err = repo.ReplaceKnowledge(ctx, tx, lessonID, jobID, points); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	replace([]course.KnowledgeDraft{p})
	points, err := repo.ListKnowledge(ctx, pool, courseID, nil)
	if err != nil || len(points) != 1 {
		t.Fatalf("points=%v err=%v", points, err)
	}
	originalID := points[0].ID
	p.Explanation = "数据必须有序"
	replace([]course.KnowledgeDraft{p})
	points, err = repo.ListKnowledge(ctx, pool, courseID, &lessonID)
	if err != nil || len(points) != 1 || points[0].ID != originalID || points[0].Explanation != p.Explanation {
		t.Fatalf("stable replacement failed: %v %v", points, err)
	}
	rec := apitest.Do(t, engine, http.MethodGet, "/api/v1/courses/"+courseID.String()+"/knowledge-points", nil, "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodGet, "/api/v1/courses/{course_id}/knowledge-points", 200, rec.Body.Bytes())
	p.Label = "边界"
	replace([]course.KnowledgeDraft{p})
	points, err = repo.ListKnowledge(ctx, pool, courseID, nil)
	if err != nil || len(points) != 1 || points[0].Label != "边界" {
		t.Fatalf("inactive point visible: %v %v", points, err)
	}
}
