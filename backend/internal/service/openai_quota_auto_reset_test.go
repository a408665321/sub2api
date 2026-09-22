package service

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIAutoResetCreditExtra(t *testing.T) {
	t.Run("历史账号默认关闭", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
		config := ResolveOpenAIAutoResetCreditConfig(account)
		require.False(t, config.Enabled)
		require.Equal(t, 1.0, config.Threshold5h)
		require.Equal(t, 1.0, config.Threshold7d)
	})

	t.Run("开启时补齐两个百分百阈值并剥离运行态", func(t *testing.T) {
		extra, err := normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey: true,
			OpenAIAutoResetCreditStateExtraKey:   map[string]any{"status": "success"},
		})
		require.NoError(t, err)
		require.Equal(t, 1.0, extra[OpenAIAutoResetCredit5hThresholdExtraKey])
		require.Equal(t, 1.0, extra[OpenAIAutoResetCredit7dThresholdExtraKey])
		require.NotContains(t, extra, OpenAIAutoResetCreditStateExtraKey)
	})

	t.Run("阈值和账号类型严格校验", func(t *testing.T) {
		_, err := normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: 0.0009,
		})
		require.Error(t, err)

		_, err = normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, true, map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey: true,
		})
		require.Error(t, err)
	})

	t.Run("Photonthinx策略校验并剔除运行态", func(t *testing.T) {
		extra, err := normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			photonthinxAutoResetCreditPolicyExtraKey: map[string]any{
				"mode":                 "observe",
				"reset_5h_enabled":     false,
				"reset_7d_enabled":     true,
				"seven_day_guard_days": 2.0,
			},
			"photonthinx_codex_window_presence":        map[string]any{"5h": map[string]any{"present": false}},
			"photonthinx_auto_reset_observation_state": map[string]any{"5h": map[string]any{"reason": "window_absent"}},
		})
		require.NoError(t, err)
		require.Contains(t, extra, photonthinxAutoResetCreditPolicyExtraKey)
		require.NotContains(t, extra, "photonthinx_codex_window_presence")
		require.NotContains(t, extra, "photonthinx_auto_reset_observation_state")

		_, err = normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			photonthinxAutoResetCreditPolicyExtraKey: map[string]any{"mode": "invalid"},
		})
		require.Error(t, err)

		_, err = normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			photonthinxAutoResetCreditPolicyExtraKey: map[string]any{"seven_day_guard_days": 7.5},
		})
		require.Error(t, err)
	})
}

func TestPreservePhotonthinxAutoResetManagedExtra(t *testing.T) {
	existing := map[string]any{
		photonthinxCodexWindowPresenceExtraKey:   map[string]any{"5h": map[string]any{"present": false}},
		photonthinxAutoResetObservationExtraKey:  map[string]any{"windows": map[string]any{"5h": map[string]any{"reason": "window_absent"}}},
		photonthinxAutoResetCreditPolicyExtraKey: map[string]any{"mode": "enforce"},
	}
	incoming := map[string]any{
		photonthinxCodexWindowPresenceExtraKey:   map[string]any{"5h": map[string]any{"present": true}},
		photonthinxAutoResetObservationExtraKey:  map[string]any{"windows": map[string]any{}},
		photonthinxAutoResetCreditPolicyExtraKey: map[string]any{"mode": "observe"},
	}

	preservePhotonthinxAutoResetManagedExtra(existing, incoming)

	require.Equal(t, existing[photonthinxCodexWindowPresenceExtraKey], incoming[photonthinxCodexWindowPresenceExtraKey])
	require.Equal(t, existing[photonthinxAutoResetObservationExtraKey], incoming[photonthinxAutoResetObservationExtraKey])
	require.Equal(t, map[string]any{"mode": "observe"}, incoming[photonthinxAutoResetCreditPolicyExtraKey])
}

func TestShouldAutoPauseOpenAIAccountByQuota_AutoResetCreditStates(t *testing.T) {
	now := time.Now().UTC()
	baseExtra := map[string]any{
		OpenAIAutoResetCreditEnabledExtraKey:     true,
		OpenAIAutoResetCredit5hThresholdExtraKey: 1.0,
		OpenAIAutoResetCredit7dThresholdExtraKey: 1.0,
		"auto_pause_5h_threshold":                0.8,
		"auto_pause_7d_disabled":                 true,
		"codex_5h_used_percent":                  90.0,
		"codex_usage_updated_at":                 now.Format(time.RFC3339),
		"codex_5h_reset_at":                      now.Add(time.Hour).Format(time.RFC3339),
	}

	t.Run("卡状态未知时暂停并触发异步查询", func(t *testing.T) {
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: cloneOpenAIAutoResetExtra(baseExtra)}
		paused, decision := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)
		require.True(t, paused)
		require.Equal(t, "quota_auto_reset_credit_check_5h", decision.reason)
	})

	t.Run("明确有卡时允许继续到用卡阈值", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		extra[OpenAIAutoResetCreditStateExtraKey] = OpenAIAutoResetCreditState{
			Status: OpenAIAutoResetStatusAvailable, AvailableCount: 1, CheckedAt: now.Format(time.RFC3339),
		}
		account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}
		paused, _ := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)
		require.False(t, paused)
	})

	t.Run("关闭5h用卡时有卡也不越过暂停阈值", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		extra["codex_5h_used_percent"] = 95.0
		extra[photonthinxCodexWindowPresenceExtraKey] = map[string]any{
			"5h": map[string]any{"present": true, "window_minutes": 300},
			"7d": map[string]any{"present": false},
		}
		extra[photonthinxAutoResetCreditPolicyExtraKey] = map[string]any{
			"mode":             "observe",
			"reset_5h_enabled": false,
			"reset_7d_enabled": true,
		}
		extra[OpenAIAutoResetCreditStateExtraKey] = OpenAIAutoResetCreditState{
			Status: OpenAIAutoResetStatusAvailable, AvailableCount: 1, CheckedAt: now.Format(time.RFC3339),
		}
		account := &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}

		paused, decision := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)

		require.True(t, paused)
		require.Equal(t, "quota_auto_reset_policy_5h_disabled", decision.reason)
	})

	t.Run("达到用卡阈值后即使有卡也退出调度", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		extra["codex_5h_used_percent"] = 100.0
		extra[OpenAIAutoResetCreditStateExtraKey] = OpenAIAutoResetCreditState{
			Status: OpenAIAutoResetStatusAvailable, AvailableCount: 1, CheckedAt: now.Format(time.RFC3339),
		}
		account := &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}
		paused, decision := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)
		require.True(t, paused)
		require.Equal(t, "quota_auto_reset_pending_5h", decision.reason)
	})

	t.Run("关闭5h用卡时耗尽也不进入待用卡", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		extra["codex_5h_used_percent"] = 100.0
		extra[photonthinxCodexWindowPresenceExtraKey] = map[string]any{
			"5h": map[string]any{"present": true, "window_minutes": 300},
			"7d": map[string]any{"present": false},
		}
		extra[photonthinxAutoResetCreditPolicyExtraKey] = map[string]any{
			"mode":             "observe",
			"reset_5h_enabled": false,
			"reset_7d_enabled": true,
		}
		account := &Account{ID: 6, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}

		paused, decision := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)

		require.True(t, paused)
		require.Equal(t, "quota_auto_reset_policy_5h_disabled", decision.reason)
	})

	t.Run("7d保护期内耗尽不进入待用卡", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		delete(extra, "auto_pause_7d_disabled")
		extra["auto_pause_5h_disabled"] = true
		extra["auto_pause_7d_threshold"] = 0.95
		extra["codex_7d_used_percent"] = 100.0
		extra["codex_7d_reset_at"] = now.Add(48 * time.Hour).Format(time.RFC3339)
		extra[photonthinxCodexWindowPresenceExtraKey] = map[string]any{
			"5h": map[string]any{"present": false},
			"7d": map[string]any{"present": true, "window_minutes": 10080},
		}
		extra[photonthinxAutoResetCreditPolicyExtraKey] = map[string]any{
			"mode":                 "observe",
			"reset_5h_enabled":     false,
			"reset_7d_enabled":     true,
			"seven_day_guard_days": 2.0,
		}
		account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}

		paused, decision := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)

		require.True(t, paused)
		require.Equal(t, "quota_auto_reset_policy_7d_guard", decision.reason)
	})

	t.Run("新策略缺少窗口标记时暂停并请求刷新", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		extra[photonthinxAutoResetCreditPolicyExtraKey] = map[string]any{
			"mode":             "observe",
			"reset_5h_enabled": false,
			"reset_7d_enabled": true,
		}
		account := &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}

		paused, decision := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)

		require.True(t, paused)
		require.Equal(t, "quota_auto_reset_policy_window_unknown", decision.reason)
	})

	t.Run("自然窗口重置后清除动态阻塞", func(t *testing.T) {
		extra := cloneOpenAIAutoResetExtra(baseExtra)
		extra["codex_5h_used_percent"] = 100.0
		extra["codex_5h_reset_at"] = now.Add(-time.Second).Format(time.RFC3339)
		extra[OpenAIAutoResetCreditStateExtraKey] = OpenAIAutoResetCreditState{
			Status: OpenAIAutoResetStatusFailed, TriggerWindow: "5h", ErrorCode: "RESET_FAILED",
		}
		account := &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}
		paused, _ := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)
		require.False(t, paused)
	})
}

func TestSelectOpenAIAutoResetCandidate_FailsClosed(t *testing.T) {
	candidates := []openAIAutoResetCreditCandidate{
		{ID: "later", ExpiresAt: "2026-09-02T00:00:00Z"},
		{ID: "earlier", ExpiresAt: "2026-09-01T00:00:00Z"},
	}
	selected, err := selectOpenAIAutoResetCandidate(candidates, 2, nil, "cycle-a")
	require.NoError(t, err)
	require.Equal(t, "earlier", selected.ID)

	_, err = selectOpenAIAutoResetCandidate([]openAIAutoResetCreditCandidate{
		{ExpiresAt: "2026-09-01T00:00:00Z"},
	}, 1, nil, "cycle-a")
	require.Error(t, err)

	_, err = selectOpenAIAutoResetCandidate(candidates, 2, &OpenAIAutoResetCreditState{
		AttemptCycleHash: "cycle-a", AttemptCreditHash: shortOpenAIAutoResetHash("missing"),
	}, "cycle-a")
	require.Error(t, err, "模糊结果后原卡消失时不得切换下一张卡")
}

func TestOpenAIQuotaAutoResetService_AssessesIndependentWindows(t *testing.T) {
	service := &OpenAIQuotaAutoResetService{}
	account := &Account{Extra: map[string]any{
		"auto_pause_5h_disabled": true,
		"auto_pause_7d_disabled": true,
	}}
	config := OpenAIAutoResetCreditConfig{Enabled: true, Threshold5h: 0.8, Threshold7d: 0.9}
	tests := []struct {
		name       string
		fiveHour   float64
		sevenDay   float64
		wantWindow string
	}{
		{name: "5h", fiveHour: 0.8, sevenDay: 0.2, wantWindow: "5h"},
		{name: "7d", fiveHour: 0.2, sevenDay: 0.9, wantWindow: "7d"},
		{name: "同时触发", fiveHour: 0.95, sevenDay: 0.95, wantWindow: "5h+7d"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment := service.buildAssessment(account, config, test.fiveHour, test.sevenDay)
			require.True(t, assessment.resetReached)
			require.Equal(t, test.wantWindow, assessment.triggerWindow)
		})
	}
}

func TestOpenAIQuotaAutoResetService_Disabled5hDoesNotTriggerReset(t *testing.T) {
	service := &OpenAIQuotaAutoResetService{}
	account := &Account{Extra: map[string]any{
		"auto_pause_5h_threshold": 0.95,
		"auto_pause_7d_disabled":  true,
		"photonthinx_auto_reset_credit_policy": map[string]any{
			"mode":             "observe",
			"reset_5h_enabled": false,
			"reset_7d_enabled": true,
		},
	}}
	config := OpenAIAutoResetCreditConfig{Enabled: true, Threshold5h: 1, Threshold7d: 1}

	assessment := service.buildAssessment(account, config, 1, 0.25)

	require.False(t, assessment.resetReached)
	require.True(t, assessment.pauseReached)
	require.Equal(t, "5h", assessment.triggerWindow)
}

func TestOpenAIQuotaAutoResetService_SevenDayGuardIncludesBoundary(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	service := &OpenAIQuotaAutoResetService{}
	account := &Account{Extra: map[string]any{
		"auto_pause_5h_disabled":       true,
		"auto_pause_7d_disabled":       true,
		"codex_7d_used_percent":        100.0,
		"codex_7d_reset_at":            now.Add(48 * time.Hour).Format(time.RFC3339),
		"codex_7d_reset_after_seconds": float64((48 * time.Hour).Seconds()),
		"codex_usage_updated_at":       now.Format(time.RFC3339),
		photonthinxCodexWindowPresenceExtraKey: map[string]any{
			"5h": map[string]any{"present": false},
			"7d": map[string]any{"present": true, "window_minutes": 10080},
		},
		photonthinxAutoResetCreditPolicyExtraKey: map[string]any{
			"mode":                 "observe",
			"reset_5h_enabled":     false,
			"reset_7d_enabled":     true,
			"seven_day_guard_days": 2.0,
		},
	}}
	config := OpenAIAutoResetCreditConfig{Enabled: true, Threshold5h: 1, Threshold7d: 1}

	assessment := service.assessExtra(account, config, now)

	require.False(t, assessment.resetReached)
	require.True(t, assessment.pauseReached)
	require.Equal(t, "7d", assessment.triggerWindow)
	require.Equal(t, "natural_reset_guard", assessment.policyReason)

	delete(account.Extra, "codex_7d_reset_after_seconds")
	assessment = service.assessExtra(account, config, now)
	require.False(t, assessment.resetReached)
	require.Equal(t, "invalid_window_signal", assessment.policyReason)
}

func TestBuildOpenAIAutoResetUsageUpdates_RecordsAbsentFiveHourWindow(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	updates := buildOpenAIAutoResetUsageUpdates(&OpenAIQuotaUsage{
		FetchedAt: now.Unix(),
		RateLimit: &OpenAIRateLimit{
			PrimaryWindow: &OpenAIRateLimitWindow{
				UsedPercent: 42, LimitWindowSeconds: 7 * 24 * 60 * 60,
				ResetAfterSeconds: int64((72 * time.Hour).Seconds()),
			},
		},
	}, now)

	presence, ok := updates[photonthinxCodexWindowPresenceExtraKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"present": false}, presence["5h"])
	require.Equal(t, map[string]any{"present": true, "window_minutes": 10080}, presence["7d"])
}

func TestBuildOpenAIAutoResetUsageUpdates_ClearsPresenceWhenRateLimitMissing(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	updates := buildOpenAIAutoResetUsageUpdates(&OpenAIQuotaUsage{FetchedAt: now.Unix()}, now)

	presence, ok := updates[photonthinxCodexWindowPresenceExtraKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"present": false}, presence["5h"])
	require.Equal(t, map[string]any{"present": false}, presence["7d"])
	require.Equal(t, now.Format(time.RFC3339), updates["codex_usage_updated_at"])
}

func TestOpenAIQuotaAutoResetService_IgnoresResidualFiveHourUsageWhenWindowAbsent(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	service := &OpenAIQuotaAutoResetService{}
	account := &Account{Extra: map[string]any{
		"codex_5h_used_percent":  100.0,
		"codex_5h_reset_at":      now.Add(time.Hour).Format(time.RFC3339),
		"codex_usage_updated_at": now.Format(time.RFC3339),
		photonthinxCodexWindowPresenceExtraKey: map[string]any{
			"5h": map[string]any{"present": false},
			"7d": map[string]any{"present": true, "window_minutes": 10080},
		},
	}}
	config := OpenAIAutoResetCreditConfig{Enabled: true, Threshold5h: 1, Threshold7d: 1}

	assessment := service.assessExtra(account, config, now)

	require.False(t, assessment.resetReached)
	require.False(t, assessment.pauseReached)
}

func TestOpenAIQuotaAutoResetService_ConfiguredPolicyRejectsUnclassifiedFreshWindow(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(109, now, "observe")
	account.Extra[photonthinxAutoResetCreditPolicyExtraKey].(map[string]any)["reset_5h_enabled"] = true
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.usage.RateLimit.PrimaryWindow.LimitWindowSeconds = 0
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Zero(t, quota.resetCalls.Load())
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		require.NotEqual(t, "would_reset", recorded.Extra["decision"])
	}
}

func TestPhotonthinxSevenDayResetRequiresValidSignalWithoutGuard(t *testing.T) {
	policy := photonthinxAutoResetCreditPolicy{
		Configured: true, Mode: "observe", Reset5hEnabled: false, Reset7dEnabled: true,
	}
	extra := map[string]any{
		photonthinxCodexWindowPresenceExtraKey: map[string]any{
			"5h": map[string]any{"present": false},
			"7d": map[string]any{"present": true, "window_minutes": 10080},
		},
		"codex_7d_reset_at":            time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"codex_7d_reset_after_seconds": 0.0,
	}

	allowed, reason := policy.sevenDayResetEligibility(extra, time.Now())
	require.False(t, allowed)
	require.Equal(t, "invalid_window_signal", reason)
}

type autoResetTestAccountRepo struct {
	AccountRepository
	mu      sync.Mutex
	account *Account
}

func (r *autoResetTestAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *r.account
	copy.Extra = cloneOpenAIAutoResetExtra(r.account.Extra)
	return &copy, nil
}

func (r *autoResetTestAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.account.Extra == nil {
		r.account.Extra = make(map[string]any)
	}
	for key, value := range updates {
		r.account.Extra[key] = value
	}
	return nil
}

type autoResetTestQuota struct {
	usage        *OpenAIQuotaUsage
	queryCalls   atomic.Int32
	onQuery      func(int32)
	resetCalls   atomic.Int32
	resetEntered chan struct{}
	releaseReset chan struct{}
	enterOnce    sync.Once
	mu           sync.Mutex
	resetArgs    [][2]string
	failFirst    bool
}

func (q *autoResetTestQuota) QueryUsage(context.Context, int64) (*OpenAIQuotaUsage, error) {
	call := q.queryCalls.Add(1)
	if q.onQuery != nil {
		q.onQuery(call)
	}
	copy := *q.usage
	return &copy, nil
}

func (q *autoResetTestQuota) CacheResetCreditsSnapshot(context.Context, int64, *OpenAIRateLimitResetCredits) error {
	return nil
}

func (q *autoResetTestQuota) CachePostResetSnapshot(context.Context, int64, *OpenAIQuotaUsage) error {
	return nil
}

func (q *autoResetTestQuota) ResetCreditTargeted(_ context.Context, _ int64, creditID, redeemRequestID string) (*OpenAIQuotaResetResult, error) {
	if creditID == "" || redeemRequestID == "" {
		panic("targeted reset identifiers must be present")
	}
	call := q.resetCalls.Add(1)
	q.mu.Lock()
	q.resetArgs = append(q.resetArgs, [2]string{creditID, redeemRequestID})
	q.mu.Unlock()
	if q.failFirst && call == 1 {
		return nil, context.DeadlineExceeded
	}
	if q.resetEntered != nil {
		q.enterOnce.Do(func() { close(q.resetEntered) })
	}
	if q.releaseReset != nil {
		<-q.releaseReset
	}
	return &OpenAIQuotaResetResult{Code: "ok", WindowsReset: 2}, nil
}

type autoResetTestRecoverer struct{}

func (autoResetTestRecoverer) RecoverAccountState(context.Context, int64, AccountRecoveryOptions) (*SuccessfulTestRecoveryResult, error) {
	return &SuccessfulTestRecoveryResult{ClearedRateLimit: true}, nil
}

func TestOpenAIQuotaAutoResetService_ObserveModeDoesNotConsumeCredit(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{
		ID: 98, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Extra: map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: 1.0,
			OpenAIAutoResetCredit7dThresholdExtraKey: 1.0,
			photonthinxAutoResetCreditPolicyExtraKey: map[string]any{
				"mode":                 "observe",
				"reset_5h_enabled":     false,
				"reset_7d_enabled":     true,
				"seven_day_guard_days": 0.0,
			},
			"codex_7d_used_percent":  100.0,
			"codex_usage_updated_at": now.Format(time.RFC3339),
			"codex_7d_reset_at":      now.Add(72 * time.Hour).Format(time.RFC3339),
			photonthinxCodexWindowPresenceExtraKey: map[string]any{
				"5h": map[string]any{"present": false},
				"7d": map[string]any{"present": true, "window_minutes": 10080},
			},
		},
	}
	repo := &autoResetTestAccountRepo{account: account}
	expiresAt := now.Add(48 * time.Hour).Format(time.RFC3339)
	quota := &autoResetTestQuota{usage: &OpenAIQuotaUsage{
		FetchedAt: now.Unix(),
		RateLimit: &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{
			UsedPercent: 100, LimitWindowSeconds: 7 * 24 * 60 * 60,
			ResetAfterSeconds: int64((72 * time.Hour).Seconds()), ResetAt: now.Add(72 * time.Hour).Unix(),
		}},
		RateLimitResetCredits: &OpenAIRateLimitResetCredits{
			AvailableCount: 1,
			Credits:        []OpenAIRateLimitResetCreditDetail{{ExpiresAt: expiresAt}},
		},
		autoResetCandidates: []openAIAutoResetCreditCandidate{{ID: "observe-credit", ExpiresAt: expiresAt}},
	}}
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Zero(t, quota.resetCalls.Load())
	require.Len(t, audit.queue, 2)
	reasons := make(map[string]string)
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		require.Equal(t, "system.openai.reset_credit.policy", recorded.Action)
		reasons[recorded.Extra["reason"].(string)] = recorded.Extra["decision"].(string)
	}
	require.Equal(t, "skipped", reasons["window_absent"])
	require.Equal(t, "would_reset", reasons["threshold_reached"])
	repo.mu.Lock()
	observation := repo.account.Extra[photonthinxAutoResetObservationExtraKey]
	repo.mu.Unlock()
	require.NotNil(t, observation)
}

func TestOpenAIQuotaAutoResetService_RecordsAbsentWindowOnce(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(101, now, "observe")
	account.Extra["auto_pause_7d_threshold"] = 0.95
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))

	var absent, wouldReset int
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		switch recorded.Extra["reason"] {
		case "window_absent":
			absent++
		case "threshold_reached":
			wouldReset++
			require.Equal(t, 0.95, recorded.Extra["auto_pause_threshold_7d"])
			windowDetails, ok := recorded.Extra["window_details"].(map[string]any)
			require.True(t, ok)
			sevenDay, ok := windowDetails["7d"].(map[string]any)
			require.True(t, ok)
			require.NotEmpty(t, sevenDay["reset_at"])
			require.Greater(t, sevenDay["remaining_seconds"].(int64), int64(0))
		}
	}
	require.Equal(t, 1, absent)
	require.Equal(t, 1, wouldReset)
}

func TestOpenAIQuotaAutoResetService_ObserveModeRejectsIncompleteCreditDetails(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(102, now, "observe")
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.usage.autoResetCandidates = nil
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Zero(t, quota.resetCalls.Load())

	var thresholdDecision, invalidDetailsDecision string
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		reason, _ := recorded.Extra["reason"].(string)
		decision, _ := recorded.Extra["decision"].(string)
		if reason == "threshold_reached" {
			thresholdDecision = decision
		}
		if reason == "credit_details_incomplete" {
			invalidDetailsDecision = decision
		}
	}
	require.Empty(t, thresholdDecision)
	require.Equal(t, "skipped", invalidDetailsDecision)
}

func TestOpenAIQuotaAutoResetService_RechecksPolicyAndUsageBeforeEnforcement(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(103, now, "enforce")
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.onQuery = func(call int32) {
		if call != 2 {
			return
		}
		repo.mu.Lock()
		policy := repo.account.Extra[photonthinxAutoResetCreditPolicyExtraKey].(map[string]any)
		policy["reset_7d_enabled"] = false
		repo.mu.Unlock()
	}
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), nil, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.GreaterOrEqual(t, quota.queryCalls.Load(), int32(2))
	require.Zero(t, quota.resetCalls.Load())
}

func TestOpenAIQuotaAutoResetService_FailsClosedWhenCreditDetailsDisappearOnRecheck(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(105, now, "enforce")
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.onQuery = func(call int32) {
		if call == 2 {
			quota.usage.RateLimitResetCredits = nil
			quota.usage.autoResetCandidates = nil
		}
	}
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), nil, nil, nil,
	)

	require.NotPanics(t, func() {
		require.Error(t, service.evaluateAccount(context.Background(), account.ID))
	})
	require.Zero(t, quota.resetCalls.Load())
	repo.mu.Lock()
	state := openAIAutoResetStateFromExtra(repo.account.Extra)
	repo.mu.Unlock()
	require.NotNil(t, state)
	require.Equal(t, OpenAIAutoResetStatusFailed, state.Status)
	require.Equal(t, "RESET_CREDIT_DETAILS_UNAVAILABLE", state.ErrorCode)
}

func TestOpenAIQuotaAutoResetService_RecordsNoCreditWhenAvailabilityChangesOnRecheck(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(108, now, "enforce")
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.onQuery = func(call int32) {
		if call == 2 {
			quota.usage.RateLimitResetCredits.AvailableCount = 0
			quota.usage.RateLimitResetCredits.Credits = nil
			quota.usage.autoResetCandidates = nil
		}
	}
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Zero(t, quota.resetCalls.Load())
	repo.mu.Lock()
	state := openAIAutoResetStateFromExtra(repo.account.Extra)
	repo.mu.Unlock()
	require.NotNil(t, state)
	require.Equal(t, OpenAIAutoResetStatusNoCredit, state.Status)
	var noCredit bool
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		noCredit = noCredit || recorded.Extra["reason"] == "no_credit"
	}
	require.True(t, noCredit)
}

func TestOpenAIQuotaAutoResetService_RefreshesMissingWindowPresenceBeforeDecision(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(104, now, "observe")
	delete(account.Extra, photonthinxCodexWindowPresenceExtraKey)
	account.Extra["codex_7d_used_percent"] = 20.0
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.usage.RateLimit.PrimaryWindow.UsedPercent = 20
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), nil, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Equal(t, int32(1), quota.queryCalls.Load())
	repo.mu.Lock()
	_, known := photonthinxCodexWindowPresent(repo.account.Extra, "5h")
	repo.mu.Unlock()
	require.True(t, known)
}

func TestOpenAIQuotaAutoResetService_RecordsBypassAndNoCreditAtPauseThreshold(t *testing.T) {
	tests := []struct {
		name         string
		available    int
		wantDecision string
		wantReason   string
		wantState    string
	}{
		{name: "有完整卡时模拟越过暂停线", available: 1, wantDecision: "would_bypass_pause", wantReason: "eligible_credit", wantState: OpenAIAutoResetStatusAvailable},
		{name: "无卡时记录并保持暂停", available: 0, wantDecision: "skipped", wantReason: "no_credit", wantState: OpenAIAutoResetStatusNoCredit},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now().UTC()
			account := newPhotonthinxAutoResetTestAccount(int64(110+i), now, "observe")
			account.Extra["auto_pause_7d_threshold"] = 0.95
			account.Extra["codex_7d_used_percent"] = 96.0
			repo := &autoResetTestAccountRepo{account: account}
			quota := newSevenDayAutoResetTestQuota(now)
			quota.usage.RateLimit.PrimaryWindow.UsedPercent = 96
			quota.usage.RateLimitResetCredits.AvailableCount = test.available
			if test.available == 0 {
				quota.usage.RateLimitResetCredits.Credits = nil
				quota.usage.autoResetCandidates = nil
			}
			idempotencyConfig := DefaultIdempotencyConfig()
			idempotencyConfig.ObserveOnly = false
			audit := NewAuditLogService(nil, nil)
			service := NewOpenAIQuotaAutoResetService(
				repo, quota, autoResetTestRecoverer{},
				NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
			)

			require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
			var found bool
			for len(audit.queue) > 0 {
				recorded := <-audit.queue
				if recorded.Extra["reason"] == test.wantReason {
					found = true
					require.Equal(t, test.wantDecision, recorded.Extra["decision"])
				}
			}
			require.True(t, found)
			repo.mu.Lock()
			state := openAIAutoResetStateFromExtra(repo.account.Extra)
			repo.mu.Unlock()
			require.NotNil(t, state)
			require.Equal(t, test.wantState, state.Status)
		})
	}
}

func TestOpenAIQuotaAutoResetService_DoesNotBypassPauseWithIncompleteCreditDetails(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(112, now, "observe")
	account.Extra["auto_pause_7d_threshold"] = 0.95
	account.Extra["codex_7d_used_percent"] = 96.0
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	quota.usage.RateLimit.PrimaryWindow.UsedPercent = 96
	quota.usage.autoResetCandidates = nil
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	service := NewOpenAIQuotaAutoResetService(
		repo, quota, autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig), audit, nil, nil,
	)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	repo.mu.Lock()
	state := openAIAutoResetStateFromExtra(repo.account.Extra)
	repo.mu.Unlock()
	require.NotNil(t, state)
	require.Equal(t, OpenAIAutoResetStatusFailed, state.Status)
	var invalidDetails bool
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		invalidDetails = invalidDetails || recorded.Extra["reason"] == "credit_details_incomplete"
	}
	require.True(t, invalidDetails)
}

func newPhotonthinxAutoResetTestAccount(id int64, now time.Time, mode string) *Account {
	return &Account{
		ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Extra: map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: 1.0,
			OpenAIAutoResetCredit7dThresholdExtraKey: 1.0,
			photonthinxAutoResetCreditPolicyExtraKey: map[string]any{
				"mode":                 mode,
				"reset_5h_enabled":     false,
				"reset_7d_enabled":     true,
				"seven_day_guard_days": 0.0,
			},
			"codex_7d_used_percent":  100.0,
			"codex_usage_updated_at": now.Format(time.RFC3339),
			"codex_7d_reset_at":      now.Add(72 * time.Hour).Format(time.RFC3339),
			photonthinxCodexWindowPresenceExtraKey: map[string]any{
				"5h": map[string]any{"present": false},
				"7d": map[string]any{"present": true, "window_minutes": 10080},
			},
		},
	}
}

func newSevenDayAutoResetTestQuota(now time.Time) *autoResetTestQuota {
	expiresAt := now.Add(48 * time.Hour).Format(time.RFC3339)
	return &autoResetTestQuota{usage: &OpenAIQuotaUsage{
		FetchedAt: now.Unix(),
		RateLimit: &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{
			UsedPercent: 100, LimitWindowSeconds: 7 * 24 * 60 * 60,
			ResetAfterSeconds: int64((72 * time.Hour).Seconds()), ResetAt: now.Add(72 * time.Hour).Unix(),
		}},
		RateLimitResetCredits: &OpenAIRateLimitResetCredits{
			AvailableCount: 1,
			Credits:        []OpenAIRateLimitResetCreditDetail{{ExpiresAt: expiresAt}},
		},
		autoResetCandidates: []openAIAutoResetCreditCandidate{{ID: "seven-day-credit", ExpiresAt: expiresAt}},
	}}
}

func TestOpenAIQuotaAutoResetService_ConcurrentInstancesConsumeOnce(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{
		ID: 99, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Extra: map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: 1.0,
			OpenAIAutoResetCredit7dThresholdExtraKey: 1.0,
			"codex_5h_used_percent":                  100.0,
			"codex_7d_used_percent":                  10.0,
			"codex_usage_updated_at":                 now.Format(time.RFC3339),
			"codex_5h_reset_at":                      now.Add(time.Hour).Format(time.RFC3339),
			"codex_7d_reset_at":                      now.Add(24 * time.Hour).Format(time.RFC3339),
		},
	}
	repo := &autoResetTestAccountRepo{account: account}
	usage := &OpenAIQuotaUsage{
		FetchedAt: now.Unix(),
		RateLimit: &OpenAIRateLimit{
			PrimaryWindow:   &OpenAIRateLimitWindow{UsedPercent: 100, LimitWindowSeconds: 5 * 60 * 60, ResetAfterSeconds: 3600, ResetAt: now.Add(time.Hour).Unix()},
			SecondaryWindow: &OpenAIRateLimitWindow{UsedPercent: 10, LimitWindowSeconds: 7 * 24 * 60 * 60, ResetAfterSeconds: 86400, ResetAt: now.Add(24 * time.Hour).Unix()},
		},
		RateLimitResetCredits: &OpenAIRateLimitResetCredits{
			AvailableCount: 1,
			Credits:        []OpenAIRateLimitResetCreditDetail{{ExpiresAt: now.Add(48 * time.Hour).Format(time.RFC3339)}},
		},
		autoResetCandidates: []openAIAutoResetCreditCandidate{{ID: "credit-sensitive-id", ExpiresAt: now.Add(48 * time.Hour).Format(time.RFC3339)}},
	}
	quota := &autoResetTestQuota{usage: usage, resetEntered: make(chan struct{}), releaseReset: make(chan struct{})}
	idempotencyRepo := newInMemoryIdempotencyRepo()
	config := DefaultIdempotencyConfig()
	config.ObserveOnly = false
	config.ProcessingTimeout = time.Second
	serviceA := NewOpenAIQuotaAutoResetService(repo, quota, autoResetTestRecoverer{}, NewIdempotencyCoordinator(idempotencyRepo, config), nil, nil, nil)
	serviceB := NewOpenAIQuotaAutoResetService(repo, quota, autoResetTestRecoverer{}, NewIdempotencyCoordinator(idempotencyRepo, config), nil, nil, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = serviceA.evaluateAccount(context.Background(), account.ID)
	}()
	<-quota.resetEntered
	go func() {
		defer wg.Done()
		_ = serviceB.evaluateAccount(context.Background(), account.ID)
	}()
	time.Sleep(50 * time.Millisecond)
	close(quota.releaseReset)
	wg.Wait()

	require.Equal(t, int32(1), quota.resetCalls.Load())
	repo.mu.Lock()
	state := openAIAutoResetStateFromExtra(repo.account.Extra)
	repo.mu.Unlock()
	require.NotNil(t, state)
	require.Equal(t, OpenAIAutoResetStatusSuccess, state.Status)
	encodedState, err := json.Marshal(state)
	require.NoError(t, err)
	require.NotContains(t, string(encodedState), "credit-sensitive-id")
}

func TestOpenAIQuotaAutoResetService_ConcurrentObservationIsAuditedOnce(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(106, now, "observe")
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	idempotencyRepo := newInMemoryIdempotencyRepo()
	config := DefaultIdempotencyConfig()
	config.ObserveOnly = false
	audit := NewAuditLogService(nil, nil)
	serviceA := NewOpenAIQuotaAutoResetService(repo, quota, autoResetTestRecoverer{}, NewIdempotencyCoordinator(idempotencyRepo, config), audit, nil, nil)
	serviceB := NewOpenAIQuotaAutoResetService(repo, quota, autoResetTestRecoverer{}, NewIdempotencyCoordinator(idempotencyRepo, config), audit, nil, nil)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			service := serviceA
			if index%2 == 1 {
				service = serviceB
			}
			_ = service.evaluateAccount(context.Background(), account.ID)
		}(i)
	}
	wg.Wait()

	var wouldReset int
	for len(audit.queue) > 0 {
		recorded := <-audit.queue
		if recorded.Extra["decision"] == "would_reset" {
			wouldReset++
		}
	}
	require.Equal(t, 1, wouldReset)
	require.Zero(t, quota.resetCalls.Load())
}

func TestOpenAIQuotaAutoResetService_ObserveToEnforceCanRedeemSameCycle(t *testing.T) {
	now := time.Now().UTC()
	account := newPhotonthinxAutoResetTestAccount(107, now, "observe")
	repo := &autoResetTestAccountRepo{account: account}
	quota := newSevenDayAutoResetTestQuota(now)
	idempotencyRepo := newInMemoryIdempotencyRepo()
	config := DefaultIdempotencyConfig()
	config.ObserveOnly = false
	service := NewOpenAIQuotaAutoResetService(repo, quota, autoResetTestRecoverer{}, NewIdempotencyCoordinator(idempotencyRepo, config), nil, nil, nil)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Zero(t, quota.resetCalls.Load())
	repo.mu.Lock()
	repo.account.Extra[photonthinxAutoResetCreditPolicyExtraKey].(map[string]any)["mode"] = "enforce"
	repo.mu.Unlock()

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Equal(t, int32(1), quota.resetCalls.Load())
}

func TestOpenAIQuotaAutoResetService_TimeoutRetryReusesRequestBody(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{
		ID: 100, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Extra: map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: 1.0,
			OpenAIAutoResetCredit7dThresholdExtraKey: 1.0,
			"codex_5h_used_percent":                  100.0,
			"codex_usage_updated_at":                 now.Format(time.RFC3339),
			"codex_5h_reset_at":                      now.Add(time.Hour).Format(time.RFC3339),
		},
	}
	repo := &autoResetTestAccountRepo{account: account}
	expiresAt := now.Add(48 * time.Hour).Format(time.RFC3339)
	quota := &autoResetTestQuota{
		failFirst: true,
		usage: &OpenAIQuotaUsage{
			FetchedAt: now.Unix(),
			RateLimit: &OpenAIRateLimit{
				PrimaryWindow: &OpenAIRateLimitWindow{UsedPercent: 100, LimitWindowSeconds: 5 * 60 * 60, ResetAfterSeconds: 3600, ResetAt: now.Add(time.Hour).Unix()},
			},
			RateLimitResetCredits: &OpenAIRateLimitResetCredits{
				AvailableCount: 1,
				Credits:        []OpenAIRateLimitResetCreditDetail{{ExpiresAt: expiresAt}},
			},
			autoResetCandidates: []openAIAutoResetCreditCandidate{{ID: "retry-credit", ExpiresAt: expiresAt}},
		},
	}
	idempotencyConfig := DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	idempotencyConfig.FailedRetryBackoff = 0
	service := NewOpenAIQuotaAutoResetService(
		repo,
		quota,
		autoResetTestRecoverer{},
		NewIdempotencyCoordinator(newInMemoryIdempotencyRepo(), idempotencyConfig),
		nil, nil, nil,
	)

	require.Error(t, service.evaluateAccount(context.Background(), account.ID))
	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	quota.mu.Lock()
	args := append([][2]string(nil), quota.resetArgs...)
	quota.mu.Unlock()
	require.Len(t, args, 2)
	require.Equal(t, args[0], args[1], "超时重试必须复用相同 credit_id 与 redeem_request_id")
}
