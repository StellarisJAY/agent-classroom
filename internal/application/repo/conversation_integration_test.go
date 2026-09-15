package repo

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

func TestConversationGetOrCreateAndMessages(t *testing.T) {
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

	conv, err := repo.GetOrCreate(context.Background(), course.ID, user.ID)
	require.NoError(t, err)
	require.NotEqual(t, types.NilID, conv.ID)
	get2, err := repo.GetOrCreate(context.Background(), course.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, conv.ID, get2.ID) // 幂等返回同一条

	content, _ := json.Marshal(types.MessageContent{Text: "什么是数组？"})
	require.NoError(t, repo.Append(context.Background(), conv.ID, types.MessageRoleUser, content, &conv.CourseID))
	tool, _ := json.Marshal(types.MessageContent{ToolCallID: "c1", Name: "highlight", Result: "success"})
	require.NoError(t, repo.Append(context.Background(), conv.ID, types.MessageRoleTool, tool, nil))

	msgs, err := repo.ListMessages(context.Background(), conv.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, types.MessageRoleUser, msgs[0].Role)
	require.NotNil(t, msgs[0].SectionID)
	var mc types.MessageContent
	require.NoError(t, json.Unmarshal(msgs[1].Content, &mc))
	require.Equal(t, "highlight", mc.Name)
}

func TestConversationGetOrCreateConcurrent(t *testing.T) {
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
		db.Exec("DELETE FROM users WHERE username LIKE '%_convrace'")
	})

	user := seedUsers(t, db, "convrace")[0]
	course := seedCourse(t, db, user.ID, "讨论课程", types.CourseStatusCompleted, false)
	repo := NewConversationRepo(db)

	const n = 8
	ids := make([]types.ID, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conv, err := repo.GetOrCreate(context.Background(), course.ID, user.ID)
			if conv != nil {
				ids[i] = conv.ID
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
	}
	for i := 1; i < n; i++ {
		require.Equal(t, ids[0], ids[i]) // 并发首提问收敛到同一会话
	}
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

	conv, err := repo.GetOrCreate(context.Background(), course.ID, user.ID)
	require.NoError(t, err)
	content, _ := json.Marshal(types.MessageContent{Text: "提问"})
	sid := sec.ID
	require.NoError(t, repo.Append(context.Background(), conv.ID, types.MessageRoleUser, content, &sid))
	require.NoError(t, db.Exec("DELETE FROM section WHERE id = ?", sec.ID).Error)

	msgs, err := repo.ListMessages(context.Background(), conv.ID)
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
	conv, err := repo.GetOrCreate(context.Background(), course.ID, user.ID)
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
