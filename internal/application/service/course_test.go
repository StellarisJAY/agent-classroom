package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

type mockCourseRepo struct {
	list func(types.ID, types.CourseListFilter) ([]types.CourseListRow, int64, error)
}

var _ types.CourseRepo = (*mockCourseRepo)(nil)

func (m *mockCourseRepo) List(_ context.Context, userID types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
	return m.list(userID, f)
}

func newCourseSvc(repo types.CourseRepo) types.CourseService {
	return NewCourseService(repo)
}

func sampleCourse(id types.ID, owner types.ID, status string, isPublic bool) types.Course {
	return types.Course{
		ID: id, OwnerID: owner, Title: "数组与循环", Description: "知识点",
		Status: status, IsPublic: isPublic,
		CreateAt: time.Now(), UpdateAt: time.Now(),
	}
}

func TestCourseListDefaults(t *testing.T) {
	uid := types.NewID()
	mine := sampleCourse(types.NewID(), uid, types.CourseStatusCompleted, false)
	var gotFilter types.CourseListFilter
	svc := newCourseSvc(&mockCourseRepo{
		list: func(userID types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			require.Equal(t, uid, userID)
			gotFilter = f
			return []types.CourseListRow{{Course: mine, ProgressStatus: types.ProgressStatusInProgress}}, 1, nil
		},
	})
	resp, err := svc.List(context.Background(), uid, nil)
	require.NoError(t, err)
	// nil req → 默认 all / 第 1 页 / 20
	require.Equal(t, types.CourseScopeAll, gotFilter.Scope)
	require.Equal(t, 1, gotFilter.Page)
	require.Equal(t, defaultCoursePageSize, gotFilter.PageSize)
	require.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Items, 1)
	it := resp.Items[0]
	require.True(t, it.Owned)
	require.Equal(t, types.ProgressStatusInProgress, it.Progress)
}

func TestCourseListFilterPassthrough(t *testing.T) {
	uid := types.NewID()
	svc := newCourseSvc(&mockCourseRepo{
		list: func(_ types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			require.Equal(t, types.CourseScopeMine, f.Scope)
			require.Equal(t, "go", f.Keyword)
			require.Equal(t, types.ProgressStatusUnstarted, f.Progress)
			require.Equal(t, 2, f.Page)
			require.Equal(t, 15, f.PageSize)
			require.Equal(t, 15, f.Offset)
			return nil, 0, nil
		},
	})
	_, err := svc.List(context.Background(), uid, &types.CourseListReq{
		Scope: types.CourseScopeMine, Keyword: "go",
		Progress: types.ProgressStatusUnstarted, Page: 2, PageSize: 15,
	})
	require.NoError(t, err)
}

func TestCourseListInvalidScopeFallsBack(t *testing.T) {
	uid := types.NewID()
	svc := newCourseSvc(&mockCourseRepo{
		list: func(_ types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			require.Equal(t, types.CourseScopeAll, f.Scope)
			return nil, 0, nil
		},
	})
	_, err := svc.List(context.Background(), uid, &types.CourseListReq{Scope: "bogus"})
	require.NoError(t, err)
}

func TestCourseListOwnedFalseForOtherOwner(t *testing.T) {
	uid := types.NewID()
	other := types.NewID()
	pub := sampleCourse(types.NewID(), other, types.CourseStatusCompleted, true)
	svc := newCourseSvc(&mockCourseRepo{
		list: func(_ types.ID, _ types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			return []types.CourseListRow{{Course: pub, ProgressStatus: types.ProgressStatusUnstarted}}, 1, nil
		},
	})
	resp, err := svc.List(context.Background(), uid, &types.CourseListReq{Scope: types.CourseScopePublic})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.False(t, resp.Items[0].Owned, "公共库他人课程 owned 应为 false")
	require.True(t, resp.Items[0].IsPublic)
}
