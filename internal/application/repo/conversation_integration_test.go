package repo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

func mustJSONRaw(v types.MessageContent) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestConversationCreateAndMessages(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, db.Exec("DELETE FROM message").Error)
	require.NoError(t, db.Exec("DELETE FROM conversation").Error)
	require.NoError(t, db.Exec("DELETE FROM section").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM message")
		db.Exec("DELETE FROM conversation")
		db.Exec("DELETE FROM section")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_conv'")
	})

	user := seedUsers(t, db, "conv")[0]
	course := seedCourse(t, db, user.ID, "讨论课程", types.CourseStatusCompleted, false)
	repo := NewConversationRepo(db)
	ctx := context.Background()

	conv, err := repo.Create(ctx, course.ID, user.ID)
	require.NoError(t, err)
	require.NotEqual(t, types.NilID, conv.ID)
	require.Equal(t, "", conv.Title)

	// 属主校验：他人取会话按 not found
	other := seedUsers(t, db, "conv")[1]
	_, err = repo.GetByID(ctx, other.ID, conv.ID)
	require.Equal(t, types.ErrNotFound, err)

	get, err := repo.GetByID(ctx, user.ID, conv.ID)
	require.NoError(t, err)
	require.Equal(t, conv.ID, get.ID)

	content, _ := json.Marshal(types.MessageContent{Text: "什么是数组？"})
	require.NoError(t, repo.Append(ctx, conv.ID, types.MessageRoleUser, content, &conv.CourseID))
	tool, _ := json.Marshal(types.MessageContent{ToolCallID: "c1", Name: "highlight", Result: "success"})
	require.NoError(t, repo.Append(ctx, conv.ID, types.MessageRoleTool, tool, nil))

	msgs, err := repo.ListMessages(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, types.MessageRoleUser, msgs[0].Role)
	require.NotNil(t, msgs[0].SectionID)
	var mc types.MessageContent
	require.NoError(t, json.Unmarshal(msgs[1].Content, &mc))
	require.Equal(t, "highlight", mc.Name)
}

func TestConversationListAndTitle(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, db.Exec("DELETE FROM message").Error)
	require.NoError(t, db.Exec("DELETE FROM conversation").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM message")
		db.Exec("DELETE FROM conversation")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_convlist'")
	})

	user := seedUsers(t, db, "convlist")[0]
	course := seedCourse(t, db, user.ID, "讨论课程", types.CourseStatusCompleted, false)
	repo := NewConversationRepo(db)
	ctx := context.Background()

	a, err := repo.Create(ctx, course.ID, user.ID)
	require.NoError(t, err)
	b, err := repo.Create(ctx, course.ID, user.ID)
	require.NoError(t, err)

	// 会话 b 先活跃，随后 a 更新（列表期望 a 在前）
	require.NoError(t, repo.Append(ctx, b.ID, types.MessageRoleUser, mustJSONRaw(types.MessageContent{Text: "b 问"}), nil))
	require.NoError(t, repo.UpdateTitle(ctx, b.ID, "b 会话标题"))
	require.NoError(t, repo.Append(ctx, a.ID, types.MessageRoleUser, mustJSONRaw(types.MessageContent{Text: "a 问"}), nil))
	require.NoError(t, repo.UpdateTitle(ctx, a.ID, "a 会话标题"))

	// title 只在为空时写入：UpdateTitle 已写过则不再覆盖
	require.NoError(t, repo.UpdateTitle(ctx, a.ID, "覆盖标题"))

	list, err := repo.ListByCourse(ctx, course.ID, user.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, a.ID, list[0].ID) // 最近活跃在前
	require.Equal(t, "a 会话标题", list[0].Title)
	require.Equal(t, "b 会话标题", list[1].Title)

	// 他人课程列表不串数据
	other := seedUsers(t, db, "convlist")[1]
	listOther, err := repo.ListByCourse(ctx, course.ID, other.ID)
	require.NoError(t, err)
	require.Empty(t, listOther)
}

func TestConversationSectionDeleteSetNull(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, db.Exec("DELETE FROM message").Error)
	require.NoError(t, db.Exec("DELETE FROM conversation").Error)
	require.NoError(t, db.Exec("DELETE FROM section").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM message")
		db.Exec("DELETE FROM conversation")
		db.Exec("DELETE FROM section")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_convdel'")
	})

	user := seedUsers(t, db, "convdel")[0]
	course := seedCourse(t, db, user.ID, "讨论课程", types.CourseStatusCompleted, false)
	sec := seedSection(t, db, course.ID)
	repo := NewConversationRepo(db)
	ctx := context.Background()

	conv, err := repo.Create(ctx, course.ID, user.ID)
	require.NoError(t, err)
	content, _ := json.Marshal(types.MessageContent{Text: "提问"})
	sid := sec.ID
	require.NoError(t, repo.Append(ctx, conv.ID, types.MessageRoleUser, content, &sid))
	require.NoError(t, db.Exec("DELETE FROM section WHERE id = ?", sec.ID).Error)

	msgs, err := repo.ListMessages(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Nil(t, msgs[0].SectionID) // section_id SET NULL
}

func TestConversationRoleEnum(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, db.Exec("DELETE FROM message").Error)
	require.NoError(t, db.Exec("DELETE FROM conversation").Error)
	require.NoError(t, db.Exec("DELETE FROM course").Error)
	t.Cleanup(func() {
		db.Exec("DELETE FROM message")
		db.Exec("DELETE FROM conversation")
		db.Exec("DELETE FROM course")
		db.Exec("DELETE FROM users WHERE username LIKE '%_convenum'")
	})

	user := seedUsers(t, db, "convenum")[0]
	course := seedCourse(t, db, user.ID, "讨论课程", types.CourseStatusCompleted, false)
	repo := NewConversationRepo(db)
	conv, err := repo.Create(context.Background(), course.ID, user.ID)
	require.NoError(t, err)

	// tool 角色必须可写入（旧库经 ensureEnumValue 补值）
	tool, _ := json.Marshal(types.MessageContent{ToolCallID: "c1", Name: "draw", Result: "success"})
	require.NoError(t, repo.Append(context.Background(), conv.ID, types.MessageRoleTool, tool, nil))
	assistant, _ := json.Marshal(types.MessageContent{Text: "好的"})
	require.NoError(t, repo.Append(context.Background(), conv.ID, types.MessageRoleAssistant, assistant, nil))
	// 非法角色必须被枚举拒绝
	bad, _ := json.Marshal(types.MessageContent{Text: "x"})
	require.Error(t, repo.Append(context.Background(), conv.ID, "invalid_role", bad, nil))
}
