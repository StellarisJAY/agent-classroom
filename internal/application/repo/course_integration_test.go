package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

func seedUsers(t *testing.T, db *gorm.DB, names ...string) []types.User {
	t.Helper()
	users := make([]types.User, 0, len(names))
	for _, n := range names {
		u := types.User{Username: n, Email: n + "@test.com", PasswordHash: "x"}
		require.NoError(t, NewUserRepo(db).Create(context.Background(), &u))
		users = append(users, u)
	}
	return users
}

func seedCourse(t *testing.T, db *gorm.DB, owner types.ID, title, status string, isPublic bool) types.Course {
	t.Helper()
	now := time.Now()
	c := types.Course{ID: types.NewID(), OwnerID: owner, Title: title, Description: "d",
		Status: status, IsPublic: isPublic, CreateAt: now, UpdateAt: now}
	require.NoError(t, db.Create(&c).Error)
	return c
}

func TestCourseListQuery(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// ensure base + course tables
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	// clean domain rows but keep schema
	require.NoError(t, db.Exec("DELETE FROM progress").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)

	users := seedUsers(t, db, "me_real", "other_real")
	me, other := users[0], users[1]
	t.Cleanup(func() {
		db.Exec("DELETE FROM progress")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_real'")
	})

	myOwn := seedCourse(t, db, me.ID, "我的私有课", types.CourseStatusCompleted, false)
	pub := seedCourse(t, db, other.ID, "他人的公共课", types.CourseStatusCompleted, true)
	_ = seedCourse(t, db, other.ID, "他人的私有课", types.CourseStatusCompleted, false)

	// 我为自己的课建立了 in_progress 进度
	p := types.Progress{ID: types.NewID(), CourseID: myOwn.ID, UserID: me.ID, Status: types.ProgressStatusInProgress}
	require.NoError(t, db.Create(&p).Error)

	repo := NewCourseRepo(db)

	// all：我的(owned) + 公共已完成 => 2
	rows, total, err := repo.List(ctx, me.ID, types.CourseListFilter{Scope: types.CourseScopeAll, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	byTitle := map[string]types.CourseListRow{}
	for _, r := range rows {
		byTitle[r.Course.Title] = r
	}
	require.Equal(t, types.ProgressStatusInProgress, byTitle["我的私有课"].ProgressStatus)
	require.Equal(t, types.ProgressStatusUnstarted, byTitle["他人的公共课"].ProgressStatus)

	// mine：只有我的
	_, tMine, err := repo.List(ctx, me.ID, types.CourseListFilter{Scope: types.CourseScopeMine, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), tMine)

	// public：仅公共已完成
	rowsPub, tPub, err := repo.List(ctx, me.ID, types.CourseListFilter{Scope: types.CourseScopePublic, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), tPub)
	require.Equal(t, pub.ID, rowsPub[0].Course.ID)

	// progress=unstarted：公共课（无进度记录）命中
	_, tUn, err := repo.List(ctx, me.ID, types.CourseListFilter{Scope: types.CourseScopeAll, Progress: types.ProgressStatusUnstarted, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), tUn)

	// progress=in_progress：我的已学习课命中
	_, tIn, err := repo.List(ctx, me.ID, types.CourseListFilter{Scope: types.CourseScopeAll, Progress: types.ProgressStatusInProgress, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), tIn)
}
