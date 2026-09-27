package service

import (
	"strings"
	"time"
)

type UserErrorDiagnosis struct {
	Code       string
	Title      string
	Reason     string
	Suggestion string
}

// UserErrorRequest 是面向终端用户的"错误请求"精简脱敏视图（白名单）。
// 严禁包含 account / api_key_prefix / upstream_endpoint / user_email 等
// 敏感或内部字段。注：message（网关标准化错误描述）与 key_name
// （用户自有 API Key 名称，KeysView 中本就可见）经产品决策对该用户开放；
// client_ip / user_agent / group_name / request_type / stream 均为该用户
// 自己请求的属性，经产品决策（2026-07-03）开放，
// 与用量明细已向用户展示自身 ip_address/user_agent/分组/类型 的口径对齐；
// error_body 仅在详情接口（GetUserErrorRequestDetail）按归属校验后返回。
type UserErrorRequest struct {
	ID                  int64     `json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	Model               string    `json:"model"`
	InboundEndpoint     string    `json:"inbound_endpoint"`
	StatusCode          int       `json:"status_code"`
	Category            string    `json:"category"`
	Platform            string    `json:"platform"`
	Message             string    `json:"message"`
	DiagnosisCode       string    `json:"diagnosis_code"`
	DiagnosisTitle      string    `json:"diagnosis_title"`
	DiagnosisReason     string    `json:"diagnosis_reason"`
	DiagnosisSuggestion string    `json:"diagnosis_suggestion"`
	KeyName             string    `json:"key_name"`
	KeyDeleted          bool      `json:"key_deleted"`
	ClientIP            string    `json:"client_ip,omitempty"`
	GroupName           string    `json:"group_name,omitempty"`
	RequestType         *int16    `json:"request_type,omitempty"`
	Stream              bool      `json:"stream"`
	UserAgent           string    `json:"user_agent,omitempty"`
}

// UserErrorRequestList 是用户错误请求分页结果。
type UserErrorRequestList struct {
	Items    []*UserErrorRequest `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// MapUserErrorCategory 把后端 error_phase + error_type 映射为用户侧粗分类码。
// 返回的是稳定的分类 code（前端做 i18n），不是展示文案。
func MapUserErrorCategory(phase, errType string) string {
	switch phase {
	case "auth":
		return "auth"
	case "routing":
		return "service_unavailable"
	case "account_auth", "upstream", "network":
		return "upstream"
	case "internal":
		return "internal"
	case "request":
		switch errType {
		case "rate_limit_error":
			return "rate_limit"
		case "billing_error", "subscription_error":
			return "quota"
		case "invalid_request_error":
			return "invalid_request"
		case "cyber_policy":
			return "cyber"
		}
	}
	return "other"
}

// DiagnoseUserError maps stable metadata to a safe diagnosis and never echoes source text.
func DiagnoseUserError(phase, errType string, status int, message string) UserErrorDiagnosis {
	phase = strings.ToLower(strings.TrimSpace(phase))
	errType = strings.ToLower(strings.TrimSpace(errType))
	text := strings.ToLower(message)
	has := func(parts ...string) bool {
		for _, part := range parts {
			if strings.Contains(text, part) {
				return true
			}
		}
		return false
	}
	d := func(code, title, reason, suggestion string) UserErrorDiagnosis {
		return UserErrorDiagnosis{Code: code, Title: title, Reason: reason, Suggestion: suggestion}
	}
	switch {
	case has("call_id", "call id"):
		return d("missing_call_id", "工具调用结果缺少关联标识", "提交的工具调用结果无法与先前的工具调用对应。", "请为每个 function_call_output 提供原工具调用返回的 call_id，并按调用顺序提交。")
	case errType == "model_not_found" || (phase == "routing" && has("model_not_found", "model not found", "model is not supported", "model unsupported", "模型未配置", "模型不支持")) || (status == 404 && has("model_not_found", "model not found", "unsupported model", "model is not supported")):
		return d("unsupported_model", "模型不可用", "请求的模型不存在，或当前分组不支持该模型。", "请检查模型名称，并选择当前分组支持的模型。")
	case has("concurrency limit exceeded for account", "account concurrency", "account concurrent", "account_concurrency"):
		return d("account_concurrency", "渠道并发已满", "当前渠道账户同时处理的请求数已达到上限。", "请稍后重试，或降低并发请求数。")
	case has("concurrency limit exceeded for user", "user concurrency", "user concurrent", "user_concurrency"):
		return d("user_concurrency", "用户并发已满", "你的同时请求数已达到并发上限。", "请等待已有请求完成后重试，并降低客户端并发。")
	case (phase == "request" || errType == "billing_error" || errType == "subscription_error" || errType == "rate_limit_error") && has("weekly usage limit", "weekly limit", "week limit", "7d limit", "7-day", "7天限额", "每周", "周限额"):
		return d("weekly_quota", "每周额度已用尽", "当前每周使用额度已达到上限。", "请等待额度周期重置，或联系管理员调整额度。")
	case (phase == "request" || errType == "billing_error" || errType == "subscription_error" || errType == "rate_limit_error") && has("daily usage limit", "daily quota", "daily limit", "day limit", "24h limit", "24-hour", "日限额", "每日"):
		return d("daily_quota", "每日额度已用尽", "当前每日使用额度已达到上限。", "请等待每日额度重置，或联系管理员调整额度。")
	case (phase == "request" || errType == "billing_error" || errType == "subscription_error" || errType == "rate_limit_error") && has("5h usage limit", "5h limit", "5-hour", "five hour", "5小时限额"):
		return d("five_hour_quota", "5 小时额度已用尽", "当前 5 小时使用额度已达到上限。", "请等待额度窗口重置后再试。")
	case has("insufficient balance", "balance is insufficient", "余额不足", "credit balance") || errType == "billing_error":
		return d("insufficient_balance", "余额不足", "账户可用余额不足以完成本次请求。", "请充值或联系管理员补充余额后重试。")
	case has("invalid api key", "api key is disabled", "api key disabled", "api key expired", "authentication failed") || (phase == "auth" && errType == "authentication_error"):
		return d("api_key_invalid", "API Key 无效或已停用", "用于本次请求的 API Key 无法通过认证。", "请检查 Key 是否正确、有效且未被停用，必要时创建新 Key。")
	case has("pending request", "pending requests", "too many pending", "request queue"):
		return d("pending_request_overload", "等待中的请求过多", "当前尚未完成的请求过多，系统暂时拒绝新请求。", "请等待已有请求完成，并减少并发或重试频率。")
	case phase == "routing" && (status == 503 || has("no allocatable", "temporarily unavailable", "no available account", "no available resource")):
		return d("no_allocatable_resource", "暂无可用服务资源", "当前没有可分配的渠道容量来处理该请求。", "请稍后重试；持续发生时可切换模型或联系管理员检查渠道容量。")
	case (phase == "upstream" || phase == "account_auth") && (status == 429 || errType == "rate_limit_error"):
		return d("upstream_rate_limit", "上游服务限流", "上游模型服务暂时限制了请求频率。", "请降低请求频率并采用指数退避后重试。")
	case phase == "request" && errType == "rate_limit_error":
		return d("user_rate_limit", "请求频率过高", "你的请求频率超过了当前限制。", "请降低请求频率，并采用指数退避后重试。")
	case phase == "network" || phase == "upstream" || phase == "account_auth" || status == 502 || status == 503 || status == 504:
		return d("upstream_failure", "上游服务异常", "上游服务或网络连接暂时异常，未能完成请求。", "请稍后重试；如持续发生，请联系管理员检查渠道状态。")
	case phase == "request" && errType == "cyber_policy":
		return d("cyber_policy", "请求触发安全策略", "请求内容不符合当前安全策略，已被拦截。", "请调整请求内容，避免受限制的内容后重试。")
	case phase == "request" && (status == 400 || status == 422):
		return d("malformed_request", "请求格式或参数有误", "请求体格式、参数或字段组合不符合接口要求。", "请对照接口文档检查 JSON 格式、必填字段和参数类型。")
	case phase == "internal" || status >= 500:
		return d("internal_error", "平台内部错误", "平台处理请求时发生内部异常。", "请稍后重试；如持续发生，请携带请求时间联系管理员。")
	default:
		return d("unknown_error", "请求未成功", "请求因暂时无法识别的原因失败。", "请检查请求参数后重试；如持续发生，请展开技术详情并联系管理员。")
	}
}

// CategoryToFilter 把用户侧分类码反向映射为后端过滤条件（plain ANY）。
// 未知分类返回两个空切片（即不施加分类过滤）。
// 注意："other" 与未知分类都走 default 返回空切片——"other" 无对应的 phase/type 组合，无法精确反查，因此等价于不过滤。
func CategoryToFilter(category string) (phases []string, errorTypes []string) {
	switch category {
	case "auth":
		return []string{"auth"}, nil
	case "service_unavailable":
		return []string{"routing"}, nil
	case "upstream":
		return []string{"account_auth", "upstream", "network"}, nil
	case "internal":
		return []string{"internal"}, nil
	case "rate_limit":
		return nil, []string{"rate_limit_error"}
	case "quota":
		return nil, []string{"billing_error", "subscription_error"}
	case "invalid_request":
		return nil, []string{"invalid_request_error"}
	case "cyber":
		return []string{"request"}, []string{"cyber_policy"}
	default:
		return nil, nil
	}
}

// ToUserErrorRequest 把内部 OpsErrorLog 裁剪为用户安全视图。
func ToUserErrorRequest(e *OpsErrorLog) *UserErrorRequest {
	if e == nil {
		return nil
	}
	model := e.RequestedModel
	if model == "" {
		model = e.Model
	}
	clientIP := ""
	if e.ClientIP != nil {
		clientIP = *e.ClientIP
	}
	diagnosis := DiagnoseUserError(e.Phase, e.Type, e.StatusCode, e.Message)
	return &UserErrorRequest{
		ID:                  e.ID,
		CreatedAt:           e.CreatedAt,
		Model:               model,
		InboundEndpoint:     e.InboundEndpoint,
		StatusCode:          e.StatusCode,
		Category:            MapUserErrorCategory(e.Phase, e.Type),
		Platform:            e.Platform,
		Message:             "", // Raw server text is intentionally withheld from personal APIs.
		DiagnosisCode:       diagnosis.Code,
		DiagnosisTitle:      diagnosis.Title,
		DiagnosisReason:     diagnosis.Reason,
		DiagnosisSuggestion: diagnosis.Suggestion,
		KeyName:             e.APIKeyName,
		KeyDeleted:          e.APIKeyDeleted,
		ClientIP:            clientIP,
		GroupName:           e.GroupName,
		RequestType:         e.RequestType,
		Stream:              e.Stream,
		UserAgent:           e.UserAgent,
	}
}

// UserErrorRequestDetail 是错误请求详情的脱敏视图(点击单行查看)。
// 在 UserErrorRequest 基础上额外暴露 error_body(上游错误响应正文)与 upstream_status_code;
// 仍严禁任何内部/敏感字段。
type UserErrorRequestDetail struct {
	UserErrorRequest
	ErrorBody          string `json:"error_body"`
	UpstreamStatusCode *int   `json:"upstream_status_code,omitempty"`
}

// ToUserErrorRequestDetail 把内部 OpsErrorLogDetail 裁剪为用户安全详情视图。
func ToUserErrorRequestDetail(e *OpsErrorLogDetail) *UserErrorRequestDetail {
	if e == nil {
		return nil
	}
	base := ToUserErrorRequest(&e.OpsErrorLog)
	return &UserErrorRequestDetail{
		UserErrorRequest:   *base,
		ErrorBody:          "", // Raw upstream bodies can contain provider/internal identifiers.
		UpstreamStatusCode: e.UpstreamStatusCode,
	}
}
