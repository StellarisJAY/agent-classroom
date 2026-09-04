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
		types.SectionTypeSlide, types.SectionTypeQuiz, types.SectionTypeDemo); err != nil {
		return err
	}
	if err := ensureEnum(db, "section_status",
		types.SectionStatusPending, types.SectionStatusGenerating, types.SectionStatusDone); err != nil {
		return err
	}
	if err := ensureEnum(db, "question_type",
		types.QuestionTypeSingle, types.QuestionTypeMultiple); err != nil {
		return err
	}

	if err := migrateCourseSchema(db); err != nil {
		return err
	}
	if err := migrateSectionSchema(db); err != nil {
		return err
	}
	return migrateQuestionSchema(db)
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
			create_by uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at timestamptz NOT NULL DEFAULT now(),
			update_by uuid REFERENCES users(id) ON DELETE SET NULL,
			update_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outline_course ON outline (course_id)`,
		`CREATE TABLE IF NOT EXISTS document (
			id        uuid PRIMARY KEY,
			course_id uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
			filename  text NOT NULL,
			url       text NOT NULL,
			create_by uuid REFERENCES users(id) ON DELETE SET NULL,
			create_at timestamptz NOT NULL DEFAULT now()
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
	if err := db.Exec(`ALTER TABLE course DROP COLUMN IF EXISTS description`).Error; err != nil {
		return fmt.Errorf("migrate course drop description: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS model_config_id uuid REFERENCES user_model_config(id) ON DELETE SET NULL`).Error; err != nil {
		return fmt.Errorf("migrate course add model_config_id: %w", err)
	}
	if err := db.Exec(`ALTER TABLE course ADD COLUMN IF NOT EXISTS thinking text NOT NULL DEFAULT 'default'`).Error; err != nil {
		return fmt.Errorf("migrate course add thinking: %w", err)
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
