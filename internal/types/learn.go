package types

import "encoding/json"

// 学习页所需的课程详情结构（GET /api/courses/:id/learn）。
// Slide 环节的 content / steps 产物在生成时已按 docs/slide数据结构.md 的 JSON
// 结构落库，此处以 json.RawMessage 原样透传给前端，避免重复定义结构。

// LearnCourse 学习详情中的课程摘要。
type LearnCourse struct {
	ID    ID     `json:"id"`
	Title string `json:"title"`
}

// SectionLearn 学习视图环节：含生成产物 content / steps。
type SectionLearn struct {
	ID              ID       `json:"id"`
	Position        int      `json:"position"`
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	KnowledgePoints []string `json:"knowledge_points"`
	Status          string   `json:"status"`
	// Content 按 type 区分的生成产物（slide: SlideContent；quiz/demo: 占位对象或 null）。
	Content json.RawMessage `json:"content"`
	// Steps slide 讲解步骤（仅 slide 使用）；其余为 null。
	Steps json.RawMessage `json:"steps"`
	// Questions quiz 题目列表（仅 quiz 使用）；其余为空数组。
	Questions []LearnQuestion `json:"questions"`
}

// CourseLearnDetail 课程学习详情。
type CourseLearnDetail struct {
	Course   LearnCourse    `json:"course"`
	Progress string         `json:"progress"`
	Sections []SectionLearn `json:"sections"`
}
