package model

// TTSVoiceTone 平台统一定义的音色。跨供应商共享同一套 ID，
// 建课时从中选择；各 TTS 适配器负责把 ID 映射为供应商实际的音色参数。
type TTSVoiceTone struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// TTSDefaultVoiceID 默认音色，未显式选择时使用。
const TTSDefaultVoiceID = "female_warm"

// TTSVoiceCatalog 平台内置音色清单。ID 为跨供应商的稳定标识（落库复用），
// Label 为面向用户的中文展示名。
var TTSVoiceCatalog = []TTSVoiceTone{
	{ID: "female_warm", Label: "女声·温暖"},
	{ID: "female_bright", Label: "女声·清亮"},
	{ID: "female_narration", Label: "女声·旁白"},
	{ID: "male_deep", Label: "男声·低沉"},
	{ID: "male_steady", Label: "男声·沉稳"},
	{ID: "neutral_clear", Label: "中性·清晰"},
}
