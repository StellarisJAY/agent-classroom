package repo

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

func seedSection(t *testing.T, db *gorm.DB, courseID types.ID) types.Section {
	t.Helper()
	kp, _ := json.Marshal([]string{"k"})
	s := types.Section{ID: types.NewID(), CourseID: courseID, Position: 1,
		Type: types.SectionTypeQuiz, Title: "测试", KnowledgePoints: datatypes.JSON(kp),
		Status: types.SectionStatusDone, CreateAt: time.Now(), UpdateAt: time.Now()}
	require.NoError(t, db.Create(&s).Error)
	return s
}

func TestQuestionReplaceAndList(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	require.NoError(t, db.Exec("DELETE FROM question").Error)
	require.NoError(t, db.Exec("DELETE FROM section").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM question")
		db.Exec("DELETE FROM section")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_qreal'")
	})

	user := seedUsers(t, db, "qreal")[0]
	course := seedCourse(t, db, user.ID, "课", types.CourseStatusCompleted, false)
	sec := seedSection(t, db, course.ID)

	repo := NewQuestionRepo(db)
	by := user.ID
	q := func(pos int, opts []string) types.Question {
		o, _ := json.Marshal(opts)
		return types.Question{Position: pos, Type: types.QuestionTypeSingle,
			Stem: "题干", Options: datatypes.JSON(o), Answers: datatypes.JSON([]byte("[0]")),
			Explanations: datatypes.JSON([]byte(`["对","错"]`)), CreateBy: &by, UpdateBy: &by}
	}
	require.NoError(t, repo.ReplaceBySection(ctx, sec.ID, []types.Question{q(1, []string{"a", "b"}), q(2, []string{"c", "d"})}))

	list, err := repo.ListBySection(ctx, sec.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, sec.ID, list[0].SectionID)
	require.Equal(t, 1, list[0].Position)
	require.Equal(t, 2, list[1].Position)
	require.NotEqual(t, types.NilID, list[0].ID)

	// 整表替换：旧题清除，仅剩新题。
	require.NoError(t, repo.ReplaceBySection(ctx, sec.ID, []types.Question{q(1, []string{"x", "y"})}))
	list2, err := repo.ListBySection(ctx, sec.ID)
	require.NoError(t, err)
	require.Len(t, list2, 1)
	require.Equal(t, 1, list2[0].Position)

	// 空替换：删除该环节全部题目。
	require.NoError(t, repo.ReplaceBySection(ctx, sec.ID, nil))
	list3, err := repo.ListBySection(ctx, sec.ID)
	require.NoError(t, err)
	require.Empty(t, list3)
}
