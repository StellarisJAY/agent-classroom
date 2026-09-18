package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// UpdateFailure 置 failed 并记录原因；UpdateStatus 置非 failed 时清空原因。
func TestSectionUpdateFailureAndClear(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	require.NoError(t, db.Exec("DELETE FROM section").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM section")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_secfail'")
	})

	user := seedUsers(t, db, "secfail")[0]
	course := seedCourse(t, db, user.ID, "失败课程", types.CourseStatusGenerating, false)
	sec := seedSection(t, db, course.ID)

	repo := NewSectionRepo(db)
	require.NoError(t, repo.UpdateFailure(ctx, sec.ID, "llm timeout"))

	list, err := repo.ListByCourse(ctx, course.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, types.SectionStatusFailed, list[0].Status)
	require.NotNil(t, list[0].FailReason)
	require.Equal(t, "llm timeout", *list[0].FailReason)

	// 重试开始（置 generating）应清空失败原因。
	require.NoError(t, repo.UpdateStatus(ctx, sec.ID, types.SectionStatusGenerating))
	list, err = repo.ListByCourse(ctx, course.ID)
	require.NoError(t, err)
	require.Equal(t, types.SectionStatusGenerating, list[0].Status)
	require.Nil(t, list[0].FailReason)

	// 重试成功后置 done，仍无失败原因。
	require.NoError(t, repo.UpdateStatus(ctx, sec.ID, types.SectionStatusDone))
	list, err = repo.ListByCourse(ctx, course.ID)
	require.NoError(t, err)
	require.Equal(t, types.SectionStatusDone, list[0].Status)
	require.Nil(t, list[0].FailReason)
	require.WithinDuration(t, time.Now(), list[0].UpdateAt, time.Minute)
}
