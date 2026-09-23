package insights

import (
	"sort"
	"time"
)

// OfficialModel is one manually curated, production-facing model entry.
// The catalog intentionally does not discover names from accounts, mappings,
// request history, or test fixtures.
type OfficialModel struct {
	Platform    string
	Name        string
	DisplayName string
	Profile     ModelProfile
}

var officialCatalogUpdatedAt = time.Date(2026, time.September, 23, 0, 0, 0, 0, time.UTC)

func officialSource(label, url string) ProfileSource {
	return ProfileSource{Label: label, URL: url}
}

func officialProfile(
	description string,
	useCases []string,
	contextLimit, maxOutput int64,
	inputModalities, outputModalities []string,
	reasoning, toolCalling, structuredOutput Capability,
	sources ...ProfileSource,
) ModelProfile {
	profile := ModelProfile{
		Description:      &description,
		UseCases:         append([]string(nil), useCases...),
		InputModalities:  append([]string(nil), inputModalities...),
		OutputModalities: append([]string(nil), outputModalities...),
		Reasoning:        reasoning,
		ToolCalling:      toolCalling,
		StructuredOutput: structuredOutput,
		Sources:          append([]ProfileSource(nil), sources...),
		UpdatedAt:        &officialCatalogUpdatedAt,
		Version:          1,
	}
	if contextLimit > 0 {
		profile.ContextLimit = &contextLimit
	}
	if maxOutput > 0 {
		profile.MaxOutput = &maxOutput
	}
	return profile
}

func addOfficialFamily(out *[]OfficialModel, platform string, profile ModelProfile, models map[string]string) {
	for name, displayName := range models {
		*out = append(*out, OfficialModel{Platform: platform, Name: name, DisplayName: displayName, Profile: profile})
	}
}

// OfficialModelCatalog returns the version-controlled allowlist used by the
// Insights model plaza. Additions require an official vendor source and code
// review; internal aliases, experiments, and test-only names do not belong here.
func OfficialModelCatalog() []OfficialModel {
	openAIReasoning := officialProfile(
		"OpenAI 通用推理模型，面向复杂知识工作、代码与智能体任务。",
		[]string{"复杂推理", "代码开发", "智能体工作流"},
		1_050_000, 128_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("OpenAI 模型文档", "https://developers.openai.com/api/docs/models"),
	)
	openAIMini := officialProfile(
		"OpenAI 轻量通用模型，在延迟、成本与推理能力之间取平衡。",
		[]string{"高并发对话", "代码辅助", "批量处理"},
		400_000, 128_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("OpenAI 模型文档", "https://developers.openai.com/api/docs/models"),
	)
	openAINano := officialProfile(
		"OpenAI 小型高吞吐模型，适合分类、抽取和简单自动化任务。",
		[]string{"分类", "信息抽取", "高吞吐自动化"},
		400_000, 128_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("OpenAI 模型文档", "https://developers.openai.com/api/docs/models"),
	)
	openAIImage := officialProfile(
		"OpenAI 图像生成与编辑模型。",
		[]string{"图像生成", "图像编辑", "视觉内容制作"},
		0, 0,
		[]string{"text", "image"}, []string{"image"},
		CapabilityUnsupported, CapabilityUnsupported, CapabilityUnsupported,
		officialSource("OpenAI 图像生成文档", "https://developers.openai.com/api/docs/guides/image-generation"),
	)
	openAIPrevious := officialProfile(
		"OpenAI 通用推理模型，适合知识工作、代码和工具驱动任务。",
		[]string{"通用推理", "代码开发", "工具调用"},
		272_000, 128_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("OpenAI 模型文档", "https://developers.openai.com/api/docs/models"),
	)
	openAICodex := officialProfile(
		"OpenAI Codex 系列代码模型，面向软件开发、代码审查与智能体式工程任务。",
		[]string{"代码开发", "代码审查", "软件工程智能体"},
		272_000, 128_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("OpenAI Codex 模型文档", "https://developers.openai.com/api/docs/models"),
	)

	out := make([]OfficialModel, 0, 64)
	addOfficialFamily(&out, "openai", openAIReasoning, map[string]string{
		"gpt-5.4":       "GPT-5.4",
		"gpt-5.5":       "GPT-5.5",
		"gpt-5.5-pro":   "GPT-5.5 Pro",
		"gpt-5.6":       "GPT-5.6",
		"gpt-5.6-sol":   "GPT-5.6 Sol",
		"gpt-5.6-terra": "GPT-5.6 Terra",
		"gpt-5.6-luna":  "GPT-5.6 Luna",
		"gpt-6-astra":   "GPT-6 Astra",
		"gpt-6":         "GPT-6 (Astra)",
	})
	addOfficialFamily(&out, "openai", openAIPrevious, map[string]string{"gpt-5.2": "GPT-5.2"})
	addOfficialFamily(&out, "openai", openAICodex, map[string]string{
		"gpt-5.2-codex":       "GPT-5.2 Codex",
		"gpt-5.3-codex":       "GPT-5.3 Codex",
		"gpt-5.3-codex-spark": "GPT-5.3 Codex Spark",
	})
	addOfficialFamily(&out, "openai", openAIMini, map[string]string{"gpt-5.4-mini": "GPT-5.4 mini"})
	addOfficialFamily(&out, "openai", openAINano, map[string]string{"gpt-5.4-nano": "GPT-5.4 nano"})
	addOfficialFamily(&out, "openai", openAIImage, map[string]string{
		"gpt-image-1":            "GPT Image 1",
		"gpt-image-1.5":          "GPT Image 1.5",
		"gpt-image-2":            "GPT Image 2",
		"gpt-image-2.5-flare":    "GPT Image 2.5 Flare",
		"gpt-image-2.5-sunburst": "GPT Image 2.5 Sunburst",
	})

	claudeLegacy := officialProfile(
		"Anthropic Claude 模型，面向长文本理解、代码和复杂知识工作。",
		[]string{"长文档分析", "代码开发", "复杂知识工作"},
		200_000, 64_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("Anthropic 模型概览", "https://platform.claude.com/docs/en/about-claude/models/overview"),
		officialSource("Anthropic 模型生命周期", "https://platform.claude.com/docs/en/about-claude/model-deprecations"),
	)
	claudeCurrent := officialProfile(
		"Anthropic 当前一代 Claude 模型，支持长上下文、多模态输入和智能体工作流。",
		[]string{"长上下文推理", "代码开发", "智能体工作流"},
		1_000_000, 128_000,
		[]string{"text", "image"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("Anthropic 模型概览", "https://platform.claude.com/docs/en/about-claude/models/overview"),
	)
	addOfficialFamily(&out, "antigravity", claudeLegacy, map[string]string{
		"claude-haiku-4-5-20251001":  "Claude Haiku 4.5",
		"claude-opus-4-5-thinking":   "Claude Opus 4.5 Thinking",
		"claude-opus-4-6":            "Claude Opus 4.6",
		"claude-opus-4-6-thinking":   "Claude Opus 4.6 Thinking",
		"claude-sonnet-4-5":          "Claude Sonnet 4.5",
		"claude-sonnet-4-5-thinking": "Claude Sonnet 4.5 Thinking",
		"claude-sonnet-4-6":          "Claude Sonnet 4.6",
		"claude-sonnet-5":            "Claude Sonnet 5",
		"claude-opus-4-7":            "Claude Opus 4.7",
		"claude-opus-4-8":            "Claude Opus 4.8",
		"claude-opus-5":              "Claude Opus 5",
	})
	addOfficialFamily(&out, "antigravity", claudeCurrent, map[string]string{
		"claude-haiku-4-8": "Claude Haiku 4.8",
		"claude-opus-5-5":  "Claude Opus 5.5",
		"claude-fable-5":   "Claude Fable 5",
		"claude-fable-5-1": "Claude Fable 5.1",
	})

	gemini := officialProfile(
		"Google Gemini 多模态模型，支持大上下文、工具调用和结构化输出。",
		[]string{"多模态理解", "长上下文分析", "智能体工作流"},
		1_048_576, 65_536,
		[]string{"text", "image", "audio", "video"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("Google Gemini 模型文档", "https://ai.google.dev/gemini-api/docs/models"),
	)
	geminiImage := officialProfile(
		"Google Gemini 图像模型，支持基于文本和图像的生成与编辑工作流。",
		[]string{"图像生成", "图像编辑", "多模态创作"},
		0, 0,
		[]string{"text", "image"}, []string{"text", "image"},
		CapabilityUnknown, CapabilityUnsupported, CapabilityUnknown,
		officialSource("Google Gemini 图像生成文档", "https://ai.google.dev/gemini-api/docs/image-generation"),
	)
	addOfficialFamily(&out, "antigravity", gemini, map[string]string{
		"gemini-2.5-flash":          "Gemini 2.5 Flash",
		"gemini-2.5-flash-lite":     "Gemini 2.5 Flash Lite",
		"gemini-2.5-flash-thinking": "Gemini 2.5 Flash Thinking",
		"gemini-3-flash":            "Gemini 3 Flash",
		"gemini-3-pro-low":          "Gemini 3 Pro Low",
		"gemini-3-pro-high":         "Gemini 3 Pro High",
		"gemini-3-pro-preview":      "Gemini 3 Pro Preview",
		"gemini-3.1-pro":            "Gemini 3.1 Pro",
		"gemini-3.1-pro-low":        "Gemini 3.1 Pro Low",
		"gemini-3.1-pro-high":       "Gemini 3.1 Pro High",
		"gemini-3.6-flash":          "Gemini 3.6 Flash",
		"gemini-3.6-flash-low":      "Gemini 3.6 Flash Low",
		"gemini-3.6-flash-medium":   "Gemini 3.6 Flash Medium",
		"gemini-3.6-flash-high":     "Gemini 3.6 Flash High",
		"gemini-3.6-flash-tiered":   "Gemini 3.6 Flash Tiered",
		"gemini-3.7-flash":          "Gemini 3.7 Flash",
		"gemini-3.7-flash-low":      "Gemini 3.7 Flash Low",
		"gemini-3.7-flash-medium":   "Gemini 3.7 Flash Medium",
		"gemini-3.7-flash-high":     "Gemini 3.7 Flash High",
		"gemini-3.7-flash-tiered":   "Gemini 3.7 Flash Tiered",
		"gemini-3.8-flash":          "Gemini 3.8 Flash",
		"gemini-3.8-flash-low":      "Gemini 3.8 Flash Low",
		"gemini-3.8-flash-medium":   "Gemini 3.8 Flash Medium",
		"gemini-3.8-flash-high":     "Gemini 3.8 Flash High",
		"gemini-3.8-flash-tiered":   "Gemini 3.8 Flash Tiered",
	})
	addOfficialFamily(&out, "antigravity", geminiImage, map[string]string{
		"gemini-2.5-flash-image":         "Gemini 2.5 Flash Image",
		"gemini-2.5-flash-image-preview": "Gemini 2.5 Flash Image Preview",
		"gemini-3-pro-image":             "Gemini 3 Pro Image",
		"gemini-3.1-flash-image":         "Gemini 3.1 Flash Image",
		"gemini-3.1-flash-image-preview": "Gemini 3.1 Flash Image Preview",
	})

	deepSeek := officialProfile(
		"DeepSeek 通用推理模型，支持长上下文、工具调用和结构化输出。",
		[]string{"复杂推理", "代码开发", "长上下文分析"},
		1_048_576, 393_216,
		[]string{"text"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("DeepSeek API 文档", "https://api-docs.deepseek.com/quick_start/pricing"),
		officialSource("DeepSeek V4.1 公告", "https://api-docs.deepseek.com/news/news260910"),
	)
	deepSeekVision := deepSeek
	deepSeekVision.InputModalities = []string{"text", "image"}
	for _, platform := range []string{"deepseek", "opencode_go"} {
		addOfficialFamily(&out, platform, deepSeek, map[string]string{
			"deepseek-flash":      "DeepSeek V4.1 Flash",
			"deepseek-v4.1-flash": "DeepSeek V4.1 Flash",
			"deepseek-v4-flash":   "DeepSeek V4 Flash",
			"deepseek-v4-pro":     "DeepSeek V4 Pro",
		})
		addOfficialFamily(&out, platform, deepSeekVision, map[string]string{
			"deepseek-v4-flash-vision-exp": "DeepSeek V4 Flash Vision Experimental",
		})
	}

	glmLegacy := officialProfile(
		"Z.AI GLM 通用语言模型，面向推理、代码、工具调用和长文本任务。",
		[]string{"通用对话", "代码开发", "工具调用"},
		200_000, 128_000,
		[]string{"text"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("Z.AI 模型文档", "https://docs.z.ai/guides/llm"),
		officialSource("Z.AI 官方定价", "https://docs.z.ai/guides/overview/pricing"),
	)
	glmCurrent := officialProfile(
		"Z.AI 新一代旗舰模型，面向长周期软件工程、代码与智能体任务。",
		[]string{"长周期代码任务", "软件工程智能体", "复杂推理"},
		1_000_000, 128_000,
		[]string{"text"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("Z.AI GLM-5.3 文档", "https://docs.z.ai/guides/llm/glm-5.3"),
		officialSource("Z.AI GLM-5.2 文档", "https://docs.z.ai/guides/llm/glm-5.2"),
	)
	glmFlash := officialProfile(
		"Z.AI 原生多模态高效模型，面向视觉编码、工具调用和专业工作流。",
		[]string{"视觉编码", "多模态理解", "智能体工作流"},
		1_000_000, 128_000,
		[]string{"text", "image", "video", "file"}, []string{"text"},
		CapabilitySupported, CapabilitySupported, CapabilitySupported,
		officialSource("Z.AI GLM-5.3-Flash 文档", "https://docs.z.ai/guides/vlm/glm-5.3-flash"),
	)
	addOfficialFamily(&out, "zhipu", glmLegacy, map[string]string{
		"glm-4.5":        "GLM-4.5",
		"glm-4.5-air":    "GLM-4.5 Air",
		"glm-4.5-flash":  "GLM-4.5 Flash",
		"glm-4.6":        "GLM-4.6",
		"glm-4.7":        "GLM-4.7",
		"glm-4.7-flash":  "GLM-4.7 Flash",
		"glm-4.7-flashx": "GLM-4.7 FlashX",
		"glm-5":          "GLM-5",
		"glm-5-turbo":    "GLM-5 Turbo",
		"glm-5.1":        "GLM-5.1",
	})
	addOfficialFamily(&out, "zhipu", glmCurrent, map[string]string{"glm-5.2": "GLM-5.2", "glm-5.3": "GLM-5.3"})
	addOfficialFamily(&out, "zhipu", glmFlash, map[string]string{"glm-5.3-flash": "GLM-5.3 Flash", "glm-5.3-flashx": "GLM-5.3 FlashX"})

	sort.Slice(out, func(i, j int) bool {
		if out[i].Platform == out[j].Platform {
			return out[i].Name < out[j].Name
		}
		return out[i].Platform < out[j].Platform
	})
	return out
}
