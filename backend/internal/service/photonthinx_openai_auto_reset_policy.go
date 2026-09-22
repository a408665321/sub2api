package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	photonthinxAutoResetCreditPolicyExtraKey = "photonthinx_auto_reset_credit_policy"
	photonthinxCodexWindowPresenceExtraKey   = "photonthinx_codex_window_presence"
	photonthinxAutoResetObservationExtraKey  = "photonthinx_auto_reset_observation_state"
)

type photonthinxAutoResetCreditPolicy struct {
	Configured        bool
	Mode              string
	Reset5hEnabled    bool
	Reset7dEnabled    bool
	SevenDayGuardDays float64
}

func resolvePhotonthinxAutoResetCreditPolicy(extra map[string]any) photonthinxAutoResetCreditPolicy {
	policy := photonthinxAutoResetCreditPolicy{
		Mode:           "enforce",
		Reset5hEnabled: true,
		Reset7dEnabled: true,
	}
	if len(extra) == 0 {
		return policy
	}
	raw, ok := extra[photonthinxAutoResetCreditPolicyExtraKey].(map[string]any)
	if !ok {
		return policy
	}
	policy.Configured = true
	if value, ok := raw["mode"].(string); ok && (value == "observe" || value == "enforce") {
		policy.Mode = value
	}
	if value, ok := raw["reset_5h_enabled"].(bool); ok {
		policy.Reset5hEnabled = value
	}
	if value, ok := raw["reset_7d_enabled"].(bool); ok {
		policy.Reset7dEnabled = value
	}
	if value, ok := resolveAccountExtraNumber(raw, "seven_day_guard_days"); ok {
		policy.SevenDayGuardDays = value
	}
	return policy
}

func normalizePhotonthinxAutoResetCreditPolicyExtra(platform, accountType string, isShadow bool, extra map[string]any) error {
	delete(extra, photonthinxCodexWindowPresenceExtraKey)
	delete(extra, photonthinxAutoResetObservationExtraKey)
	rawValue, present := extra[photonthinxAutoResetCreditPolicyExtraKey]
	if !present {
		return nil
	}
	if platform != PlatformOpenAI || accountType != AccountTypeOAuth || isShadow {
		return infraerrors.New(http.StatusBadRequest, "PHOTONTHINX_AUTO_RESET_POLICY_ACCOUNT_INVALID", "Photonthinx reset policy is only supported for OpenAI OAuth parent accounts")
	}
	raw, ok := rawValue.(map[string]any)
	if !ok {
		return infraerrors.New(http.StatusBadRequest, "PHOTONTHINX_AUTO_RESET_POLICY_INVALID", "Photonthinx reset policy must be an object")
	}
	normalized := cloneOpenAIAutoResetExtra(raw)
	if mode, exists := normalized["mode"]; exists {
		value, ok := mode.(string)
		if !ok || (value != "observe" && value != "enforce") {
			return infraerrors.New(http.StatusBadRequest, "PHOTONTHINX_AUTO_RESET_POLICY_MODE_INVALID", "mode must be observe or enforce")
		}
	}
	for _, key := range []string{"reset_5h_enabled", "reset_7d_enabled"} {
		if value, exists := normalized[key]; exists {
			if _, ok := value.(bool); !ok {
				return infraerrors.Newf(http.StatusBadRequest, "PHOTONTHINX_AUTO_RESET_POLICY_WINDOW_INVALID", "%s must be a boolean", key)
			}
		}
	}
	if _, exists := normalized["seven_day_guard_days"]; exists {
		value, ok := resolveAccountExtraNumber(normalized, "seven_day_guard_days")
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 7 {
			return infraerrors.New(http.StatusBadRequest, "PHOTONTHINX_AUTO_RESET_POLICY_GUARD_INVALID", "seven_day_guard_days must be between 0 and 7")
		}
		normalized["seven_day_guard_days"] = value
	}
	extra[photonthinxAutoResetCreditPolicyExtraKey] = normalized
	return nil
}

func stripPhotonthinxAutoResetCreditManagedExtra(extra map[string]any, stripConfig bool) {
	delete(extra, photonthinxCodexWindowPresenceExtraKey)
	delete(extra, photonthinxAutoResetObservationExtraKey)
	if stripConfig {
		delete(extra, photonthinxAutoResetCreditPolicyExtraKey)
	}
}

func preservePhotonthinxAutoResetManagedExtra(existing, target map[string]any) {
	if target == nil {
		return
	}
	for _, key := range []string{
		photonthinxCodexWindowPresenceExtraKey,
		photonthinxAutoResetObservationExtraKey,
	} {
		if value, ok := existing[key]; ok {
			target[key] = value
		} else {
			delete(target, key)
		}
	}
}

func buildPhotonthinxCodexWindowPresence(snapshot *OpenAICodexUsageSnapshot) map[string]any {
	presence := map[string]any{
		"5h": map[string]any{"present": false},
		"7d": map[string]any{"present": false},
	}
	if snapshot == nil {
		return presence
	}
	normalized := snapshot.Normalize()
	if normalized == nil {
		return presence
	}
	if minutes := normalized.Window5hMinutes; minutes != nil && *minutes >= 4*60 && *minutes <= 6*60 {
		presence["5h"] = map[string]any{"present": true, "window_minutes": *minutes}
	}
	if minutes := normalized.Window7dMinutes; minutes != nil && *minutes >= 6*24*60 && *minutes <= 8*24*60 {
		presence["7d"] = map[string]any{"present": true, "window_minutes": *minutes}
	}
	return presence
}

func photonthinxCodexWindowPresent(extra map[string]any, window string) (bool, bool) {
	rawPresence, ok := extra[photonthinxCodexWindowPresenceExtraKey].(map[string]any)
	if !ok {
		return false, false
	}
	rawWindow, ok := rawPresence[window].(map[string]any)
	if !ok {
		return false, false
	}
	present, ok := rawWindow["present"].(bool)
	return present, ok
}

func photonthinxCodexWindowPresenceComplete(extra map[string]any) bool {
	_, known5h := photonthinxCodexWindowPresent(extra, "5h")
	_, known7d := photonthinxCodexWindowPresent(extra, "7d")
	return known5h && known7d
}

func (s *OpenAIQuotaAutoResetService) persistPhotonthinxObservation(
	ctx context.Context,
	accountID int64,
	extra map[string]any,
	assessment openAIAutoResetAssessment,
	available int,
	mode string,
	decisionName string,
	reason string,
	decisionKey string,
	now time.Time,
) error {
	windows := make(map[string]any)
	if existing, ok := extra[photonthinxAutoResetObservationExtraKey].(map[string]any); ok {
		if existingWindows, ok := existing["windows"].(map[string]any); ok {
			for key, value := range existingWindows {
				windows[key] = value
			}
		}
	}
	for _, window := range strings.Split(assessment.triggerWindow, "+") {
		if window != "5h" && window != "7d" {
			continue
		}
		utilization := assessment.utilization5h
		threshold := assessment.threshold5h
		if window == "7d" {
			utilization = assessment.utilization7d
			threshold = assessment.threshold7d
		}
		decision := map[string]any{
			"mode":            mode,
			"decision":        decisionName,
			"reason":          reason,
			"decision_key":    decisionKey,
			"evaluated_at":    now.UTC().Format(time.RFC3339),
			"utilization":     utilization,
			"threshold":       threshold,
			"available_count": available,
		}
		if resetAt, ok := openAICodexWindowResetAt(extra, window); ok {
			decision["reset_at"] = resetAt.UTC().Format(time.RFC3339)
			remaining := int64(resetAt.Sub(now).Seconds())
			if remaining < 0 {
				remaining = 0
			}
			decision["remaining_seconds"] = remaining
		}
		windows[window] = decision
	}
	return s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{
		photonthinxAutoResetObservationExtraKey: map[string]any{"windows": windows},
	})
}

func (s *OpenAIQuotaAutoResetService) recordPhotonthinxObservation(
	ctx context.Context,
	accountID int64,
	extra map[string]any,
	policy photonthinxAutoResetCreditPolicy,
	assessment openAIAutoResetAssessment,
	available int,
	decisionName string,
	reason string,
	cycleHash string,
	now time.Time,
) error {
	policyHash := shortOpenAIAutoResetHash(fmt.Sprintf("%s|%t|%t|%.4f", policy.Mode, policy.Reset5hEnabled, policy.Reset7dEnabled, policy.SevenDayGuardDays))
	decisionKey := shortOpenAIAutoResetHash(fmt.Sprintf("%d|%s|%s|%s|%s|%s", accountID, assessment.triggerWindow, cycleHash, decisionName, reason, policyHash))
	if photonthinxObservationDecisionAlreadyRecorded(extra, assessment.triggerWindow, decisionKey) {
		return nil
	}
	key := fmt.Sprintf("oaro:%d:%s:%s:%s:%s:%s", accountID, assessment.triggerWindow, cycleHash, decisionName, reason, policyHash)
	if s.idempotency == nil {
		return nil
	}
	_, err := s.idempotency.Execute(ctx, IdempotencyExecuteOptions{
		Scope:          "openai_auto_reset_observation",
		ActorScope:     fmt.Sprintf("account:%d", accountID),
		Method:         "SYSTEM",
		Route:          "/system/openai/reset-credit/policy",
		IdempotencyKey: key,
		Payload: map[string]any{
			"account_id":  accountID,
			"window":      assessment.triggerWindow,
			"cycle_hash":  cycleHash,
			"decision":    decisionName,
			"reason":      reason,
			"policy_hash": policyHash,
		},
		TTL:        15 * 24 * time.Hour,
		RequireKey: true,
	}, func(execCtx context.Context) (any, error) {
		if err := s.persistPhotonthinxObservation(execCtx, accountID, extra, assessment, available, policy.Mode, decisionName, reason, decisionKey, now); err != nil {
			return nil, err
		}
		if s.audit != nil {
			windowDetails := make(map[string]any)
			for _, window := range strings.Split(assessment.triggerWindow, "+") {
				if resetAt, ok := openAICodexWindowResetAt(extra, window); ok {
					remaining := int64(resetAt.Sub(now).Seconds())
					if remaining < 0 {
						remaining = 0
					}
					windowDetails[window] = map[string]any{
						"reset_at": resetAt.UTC().Format(time.RFC3339), "remaining_seconds": remaining,
					}
				}
			}
			s.audit.Record(&AuditLog{
				ActorEmail: "system", ActorRole: "system", AuthMethod: "system",
				Action: "system.openai.reset_credit.policy", Method: "SYSTEM",
				Path:       fmt.Sprintf("/system/openai/accounts/%d/reset-credit-policy", accountID),
				StatusCode: http.StatusOK,
				Extra: map[string]any{
					"account_id": accountID, "mode": policy.Mode,
					"window": assessment.triggerWindow, "decision": decisionName,
					"reason": reason, "evaluated_at": now.UTC().Format(time.RFC3339),
					"utilization_5h": assessment.utilization5h, "utilization_7d": assessment.utilization7d,
					"threshold_5h": assessment.threshold5h, "threshold_7d": assessment.threshold7d,
					"auto_pause_threshold_5h": assessment.pauseThreshold5h, "auto_pause_threshold_7d": assessment.pauseThreshold7d,
					"guard_days": policy.SevenDayGuardDays, "available_count": available,
					"cycle_hash": cycleHash, "window_details": windowDetails,
				},
			})
		}
		return map[string]any{"decision": decisionName}, nil
	})
	return err
}

func photonthinxObservationDecisionAlreadyRecorded(extra map[string]any, triggerWindow, decisionKey string) bool {
	state, ok := extra[photonthinxAutoResetObservationExtraKey].(map[string]any)
	if !ok {
		return false
	}
	windows, ok := state["windows"].(map[string]any)
	if !ok {
		return false
	}
	for _, window := range strings.Split(triggerWindow, "+") {
		decision, ok := windows[window].(map[string]any)
		if !ok || fmt.Sprint(decision["decision_key"]) != decisionKey {
			return false
		}
	}
	return triggerWindow != ""
}

func (s *OpenAIQuotaAutoResetService) recordPhotonthinxAbsentWindows(
	ctx context.Context,
	account *Account,
	policy photonthinxAutoResetCreditPolicy,
	config OpenAIAutoResetCreditConfig,
	available int,
	now time.Time,
) error {
	if account == nil || !policy.Configured {
		return nil
	}
	for _, window := range []string{"5h", "7d"} {
		present, known := photonthinxCodexWindowPresent(account.Extra, window)
		if !known || present {
			continue
		}
		assessment := openAIAutoResetAssessment{
			triggerWindow: window,
			threshold5h:   config.Threshold5h,
			threshold7d:   config.Threshold7d,
		}
		if err := s.recordPhotonthinxObservation(
			ctx, account.ID, account.Extra, policy, assessment, available,
			"skipped", "window_absent", "unknown", now,
		); err != nil {
			return err
		}
		fresh, err := s.accountRepo.GetByID(ctx, account.ID)
		if err != nil {
			return err
		}
		if fresh != nil {
			account.Extra = fresh.Extra
		}
	}
	return nil
}

func photonthinxCreditDetailsReason(err error) string {
	if err == nil {
		return ""
	}
	switch infraerrors.Reason(err) {
	case "OPENAI_AUTO_RESET_CREDIT_DETAILS_INCOMPLETE",
		"OPENAI_AUTO_RESET_CREDIT_EXPIRY_INVALID",
		"OPENAI_AUTO_RESET_CREDIT_ID_MISSING",
		"OPENAI_AUTO_RESET_ORIGINAL_CREDIT_UNAVAILABLE":
		return "credit_details_incomplete"
	default:
		return "credit_details_invalid"
	}
}

func (p photonthinxAutoResetCreditPolicy) sevenDayResetEligibility(extra map[string]any, now time.Time) (bool, string) {
	if !p.Reset7dEnabled {
		return false, "window_disabled"
	}
	if allowed, reason := p.windowSignalEligibility(extra, "7d", now); !allowed {
		return false, reason
	}
	if p.SevenDayGuardDays <= 0 {
		return true, ""
	}
	resetAt, _ := openAICodexWindowResetAt(extra, "7d")
	guard := time.Duration(p.SevenDayGuardDays * float64(24*time.Hour))
	if resetAt.Sub(now) <= guard {
		return false, "natural_reset_guard"
	}
	return true, ""

}

func (p photonthinxAutoResetCreditPolicy) fiveHourResetEligibility(extra map[string]any, now time.Time) (bool, string) {
	if !p.Reset5hEnabled {
		return false, "window_disabled"
	}
	return p.windowSignalEligibility(extra, "5h", now)
}

func (p photonthinxAutoResetCreditPolicy) windowSignalEligibility(extra map[string]any, window string, now time.Time) (bool, string) {
	if !p.Configured {
		return true, ""
	}
	present, known := photonthinxCodexWindowPresent(extra, window)
	if !known || !present {
		return false, "invalid_window_signal"
	}
	resetAfter, ok := resolveAccountExtraNumber(extra, "codex_"+window+"_reset_after_seconds")
	if !ok || resetAfter <= 0 {
		return false, "invalid_window_signal"
	}
	resetAt, ok := openAICodexWindowResetAt(extra, window)
	if !ok || !resetAt.After(now) {
		return false, "invalid_window_signal"
	}
	return true, ""
}

func (p photonthinxAutoResetCreditPolicy) allowsSevenDayReset(extra map[string]any, now time.Time) bool {
	allowed, _ := p.sevenDayResetEligibility(extra, now)
	return allowed
}
