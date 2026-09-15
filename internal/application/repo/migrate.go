package repo

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// Migrate 建表。PostgreSQL 无法用 AutoMigrate 直接创建 ENUM 类型，
// 因此此处先以幂等方式建 ENUM，再用 AutoMigrate 建其余占位表，
// 而 course / progress 表走原始 SQL（含外键与唯一索引），
// 对齐 docs/数据库设计.md。
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&types.User{}, &types.UserModelConfig{}); err != nil {
		return fmt.Errorf("auto migrate base tables: %w", err)
	}

	if err := ensureEnum(db, "course_status",
		types.CourseStatusDraft, types.CourseStatusOutlineConfirmed,
		types.CourseStatusGenerating, types.CourseStatusCompleted); err != nil {
		return err
	}
	if err := ensureEnum(db, "progress_status",
		types.ProgressStatusUnstarted, types.ProgressStatusInProgress, types.ProgressStatusCompleted); err != nil {
		return err
	}
	if err := ensureEnum(db, "section_type",
		types.SectionTypeSlide, types.SectionTypeQuiz,
		types.SectionTypeDemo3D, types.SectionTypeDemoFunction, types.SectionTypeDemoBasic); err != nil {
		return err
	}
	// 兼容已初始化过的旧库：原有 section_type 枚举只含 slide/quiz/demo，
	// 为不存在的三种 demo 类型逐个幂等补充新值（不涉及数据迁移）。
	for _, v := range []string{
		types.SectionTypeDemo3D, types.SectionTypeDemoFunction, types.SectionTypeDemoBasic,
	} {
		if err := ensureEnumValue(db, "section_type", v); err != nil {
			return err
		}
	}
	if err := ensureEnum(db, "section_status",
		types.SectionStatusPending, types.SectionStatusGenerating, types.SectionStatusDone); err != nil {
		return err
	}
	if err := ensureEnum(db, "question_type",
		types.QuestionTypeSingle, types.QuestionTypeMultiple); err != nil {
		return err
	}
	if err := ensureEnum(db, "message_role",
		types.MessageRoleUser, types.MessageRoleAssistant, types.MessageRoleTool); err != nil {
		return err
	}
	// 兼容已初始化过的旧库：message_role 原只含 user/assistant，讨论模式补 tool。
	if err := ensureEnumValue(db, "message_role", types.MessageRoleTool); err != nil {
		return err
	}

	if err := migrateCourseSchema(db); err != nil {
		return err
	}
	if err := migrateSectionSchema(db); err != nil {
		return err
	}
	if err := migrateQuestionSchema(db); err != nil {
		return err
	}
	return migrateConversationSchema(db)
}

// ensureEnum 幂等创建 PostgreSQL ENUM 类型。
func ensureEnum(db *gorm.DB, name string, values ...string) error {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, "'"+v+"'")
	}
	ddl := fmt.Sprintf(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = '%s') THEN
			CREATE TYPE %s AS ENUM (%s);
		END IF;
	END $$;`, name, name, strings.Join(quoted, ", "))
	if err := db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("create enum %s: %w", name, err)
	}
	return nil
}

// ensureEnumValue 幂等为已存在的 ENUM 补充单个值；值已存在则跳过。
// 用于扩展已初始化库的枚举，不重建类型（不涉及数据变更）。
func ensureEnumValue(db *gorm.DB, name, value string) error {
	ddl := fmt.Sprintf(`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM pg_enum e
			JOIN pg_type t ON t.oid = e.enumtypid
			WHERE t.typname = '%s' AND e.enumlabel = '%s'
		) THEN
			ALTER TYPE %s ADD VALUE '%s';
		END IF;
	END $$;`, name, value, name, value)
	if err := db.Exec(ddl).Error; err != nil {
		return fmt.Errorf("extend enum %s with %s: %w", name, value, err)
	}
	return nil
}

// migrateCourseSchema 创建 course / progress / outline / document 表。
func migrateCourseSchema(db *gorm.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS course (
			id              uuid PRIMARY KEY,
			owner_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title           text NOT NULL DEFAULT '',
			prompt          text NOT NULL DEFAULT '',
			status          course_status NOT NULL DEFAULT 'draft',
			is_public       boolean NOT NULL DEFAULT false,
			model_config_id uuid REFERENCES user_model_config(id) ON DELETE SET NULL,
			thinking        text NOT NULL DEFAULT 'default',
			outline_count   integer NOT NULL DEFAULT 5,
			create_by       uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at       timestamptz NOT NULL DEFAULT now(),
			update_at       timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_course_owner ON course (owner_id)`,
		`CREATE TABLE IF NOT EXISTS progress (
			id        uuid PRIMARY KEY,
			course_id uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
			user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			status    progress_status NOT NULL DEFAULT 'unstarted',
			create_at timestamptz NOT NULL DEFAULT now(),
			update_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_progress_course_user ON progress (course_id, user_id)`,
		`CREATE TABLE IF NOT EXISTS outline (
			id        uuid PRIMARY KEY,
			course_id uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
			content   jsonb NOT NULL,
			status    text NOT NULL DEFAULT 'draft',
			version   integer NOT NULL DEFAULT 1,
			create_by uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at timestamptz NOT NULL DEFAULT now(),
			update_by uuid REFERENCES users(id) ON DELETE SET NULL,
			update_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outline_course ON outline (course_id)`,
		`CREATE TABLE IF NOT EXISTS outline_history (
			id         uuid PRIMARY KEY,
			outline_id uuid NOT NULL REFERENCES outline(id) ON DELETE CASCADE,
			version    integer NOT NULL,
			title      text NOT NULL DEFAULT '',
			content    jsonb NOT NULL,
			feedback   text NOT NULL DEFAULT '',
			create_at  timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_outline_history_version ON outline_history (outline_id, version)`,
		`CREATE TABLE IF NOT EXISTS document (
			id              uuid PRIMARY KEY,
			course_id       uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
			filename        text NOT NULL,
			url             text NOT NULL,
			extracted_status varchar(16) NOT NULL DEFAULT 'pending',
			extracted_text  text,
			create_by       uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at       timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_document_course ON document (course_id)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrate course schema: %w", err)
		}
	}

	// 兼容已存在的旧库：补 prompt 列、移除废弃的 description 列。
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS prompt text NOT NULL DEFAULT ''`).Error; err != nil {
		return fmt.Errorf("migrate course add prompt: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS outline_count integer NOT NULL DEFAULT 5`).Error; err != nil {
		return fmt.Errorf("migrate course add outline_count: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course DROP COLUMN IF EXISTS description`).Error; err != nil {
		return fmt.Errorf("migrate course drop description: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS model_config_id uuid REFERENCES user_model_config(id) ON DELETE SET NULL`).Error; err != nil {
		return fmt.Errorf("migrate course add model_config_id: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS generate_images boolean NOT NULL DEFAULT false`).Error; err != nil {
		return fmt.Errorf("migrate course add generate_images: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS image_model_config_id uuid REFERENCES user_model_config(id) ON DELETE SET NULL`).Error; err != nil {
		return fmt.Errorf("migrate course add image_model_config_id: %w", err)
	}
	// 默认模型不变量：同一 (user, kind) 至多一个默认。先清理历史重复默认，再建部分唯一索引。
	if err := db.Exec(`UPDATE user_model_config u SET is_default = false
		WHERE is_default AND EXISTS (
			SELECT 1 FROM user_model_config o
			WHERE o.user_id = u.user_id AND o.kind = u.kind AND o.is_default
			  AND o.id <> u.id AND o.create_at > u.create_at
		)`).Error; err != nil {
		return fmt.Errorf("migrate dedup user_model_config default: %w", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uniq_user_model_config_default
		ON user_model_config (user_id, kind) WHERE is_default`).Error; err != nil {
		return fmt.Errorf("migrate user_model_config default index: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS thinking text NOT NULL DEFAULT 'default'`).Error; err != nil {
		return fmt.Errorf("migrate course add thinking: %w", err)
	}
	// 兼容已存在的旧库：outline 补 version 列。
	if err := db.Exec(`ALTER TABLE outline ADD COLUMN IF NOT EXISTS version integer NOT NULL DEFAULT 1`).Error; err != nil {
		return fmt.Errorf("migrate outline add version: %w", err)
	}
	// 参考文档提取结果缓存：status 幂等提取 + text 复用（旧库补列）。
	if err := db.Exec(`ALTER TABLE document ADD COLUMN IF NOT EXISTS extracted_status varchar(16) NOT NULL DEFAULT 'pending'`).Error; err != nil {
		return fmt.Errorf("migrate document add extracted_status: %w", err)
	}
	if err := db.Exec(`ALTER TABLE document ADD COLUMN IF NOT EXISTS extracted_text text`).Error; err != nil {
		return fmt.Errorf("migrate document add extracted_text: %w", err)
	}
	return nil
}

// migrateSectionSchema 创建 section 表。
func migrateSectionSchema(db *gorm.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS section (
			id               uuid PRIMARY KEY,
			course_id        uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
			position         integer NOT NULL,
			type             section_type NOT NULL,
			title            text NOT NULL DEFAULT '',
			knowledge_points jsonb NOT NULL DEFAULT '[]',
			prompt           text,
			status           section_status NOT NULL DEFAULT 'pending',
			content          jsonb,
			steps            jsonb,
			create_by        uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at        timestamptz NOT NULL DEFAULT now(),
			update_by        uuid REFERENCES users(id) ON DELETE SET NULL,
			update_at        timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_section_course_position ON section (course_id, position)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrate section schema: %w", err)
		}
	}
	return nil
}

// migrateQuestionSchema 创建 question（测试题）表。对齐 docs/数据库设计.md 3.6。
func migrateQuestionSchema(db *gorm.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS question (
			id           uuid PRIMARY KEY,
			section_id   uuid NOT NULL REFERENCES section(id) ON DELETE CASCADE,
			position     integer NOT NULL,
			type         question_type NOT NULL,
			stem         text NOT NULL,
			options      jsonb NOT NULL,
			answers      jsonb NOT NULL,
			explanations jsonb NOT NULL,
			create_by    uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at    timestamptz NOT NULL DEFAULT now(),
			update_by    uuid REFERENCES users(id) ON DELETE SET NULL,
			update_at    timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_question_section_position ON question (section_id, position)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrate question schema: %w", err)
		}
	}
	return nil
}

// migrateConversationSchema 创建 conversation / message 表。对齐 docs/数据库设计.md 3.8 / 3.9
//（message.content 按讨论模式方案 §6 采用 jsonb）。
func migrateConversationSchema(db *gorm.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS conversation (
			id        uuid PRIMARY KEY,
			course_id uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
			user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			create_by uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at timestamptz NOT NULL DEFAULT now(),
			update_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_conversation_course_user ON conversation (course_id, user_id)`,
		`CREATE TABLE IF NOT EXISTS message (
			id              uuid PRIMARY KEY,
			conversation_id uuid NOT NULL REFERENCES conversation(id) ON DELETE CASCADE,
			role            message_role NOT NULL,
			content         jsonb NOT NULL,
			section_id      uuid REFERENCES section(id) ON DELETE SET NULL,
			created_at      timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_message_conversation_created ON message (conversation_id, created_at)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrate conversation schema: %w", err)
		}
	}
	return nil
}
