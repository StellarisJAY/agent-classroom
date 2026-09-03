package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

func cleanGenDomain(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("DELETE FROM document").Error)
	require.NoError(t, db.Exec("DELETE FROM outline").Error)
	require.NoError(t, db.Exec("DELETE FROM progress").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
}

func TestOutlineRepoCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(db))
	cleanGenDomain(t, db)

	users := seedUsers(t, db, "gen_outline")
	course := seedCourse(t, db, users[0].ID, "课", types.CourseStatusDraft, false)
	o := &types.Outline{
		CourseID: course.ID,
		Content:  datatypes.JSON([]byte(`{"sections":[{"title":"a","type":"slide","knowledge_points":["x"]}]}`)),
		Status:   types.OutlineStatusDraft,
	}
	require.NoError(t, NewOutlineRepo(db).Create(ctx, o))
	require.NotEqual(t, types.NilID, o.ID)

	got, err := NewOutlineRepo(db).GetByCourse(ctx, course.ID)
	require.NoError(t, err)
	require.Equal(t, o.ID, got.ID)
	require.Contains(t, string(got.Content), "slide")

	// 覆盖更新
	newContent := datatypes.JSON([]byte(`{"sections":[]}`))
	require.NoError(t, NewOutlineRepo(db).UpdateContentStatus(ctx, course.ID, newContent, types.OutlineStatusConfirmed, &users[0].ID))
	got2, err := NewOutlineRepo(db).GetByCourse(ctx, course.ID)
	require.NoError(t, err)
	require.Equal(t, types.OutlineStatusConfirmed, got2.Status)
}

func TestDocumentRepoCRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(db))
	cleanGenDomain(t, db)

	users := seedUsers(t, db, "gen_doc")
	course := seedCourse(t, db, users[0].ID, "课", types.CourseStatusDraft, false)
	repo := NewDocumentRepo(db)
	d1 := &types.Document{CourseID: course.ID, Filename: "a.md", URL: "/uploads/x/a.md"}
	require.NoError(t, repo.Create(ctx, d1))
	require.NotEqual(t, types.NilID, d1.ID)
	d2 := &types.Document{CourseID: course.ID, Filename: "b.txt", URL: "/uploads/x/b.txt"}
	require.NoError(t, repo.Create(ctx, d2))

	list, err := repo.ListByCourse(ctx, course.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
}
