//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSessionOccupancyKeysShareRedisClusterSlot(t *testing.T) {
	const accountID int64 = 701
	primary := sessionLimitKey(accountID)
	owner := sessionOccupancyKey(accountID)
	require.Contains(t, owner, "{"+primary+"}")
}

func TestSessionOccupancyGroupsOwnersAndCleansMetadata(t *testing.T) {
	cache := NewSessionLimitCache(testRedis(t), 5)
	occupancy, ok := cache.(service.SessionOccupancyCache)
	require.True(t, ok)

	ctx := context.Background()
	const accountID int64 = 701
	for _, session := range []struct {
		id     string
		userID int64
	}{{"session-a", 11}, {"session-b", 11}, {"session-c", 22}} {
		allowed, err := occupancy.RegisterSessionWithOwner(ctx, accountID, session.id, 10, 5*time.Minute, session.userID)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	owners, err := occupancy.GetSessionOccupants(ctx, accountID, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, owners, 2)
	require.Equal(t, int64(11), *owners[0].UserID)
	require.Equal(t, 2, owners[0].SessionCount)
	require.False(t, owners[0].LastActive.IsZero())
	require.Equal(t, int64(22), *owners[1].UserID)
	require.Equal(t, 1, owners[1].SessionCount)

	require.NoError(t, cache.UnregisterSession(ctx, accountID, "session-a"))
	owners, err = occupancy.GetSessionOccupants(ctx, accountID, 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, owners[0].SessionCount)
	metadata, err := cache.(*sessionLimitCache).rdb.HExists(ctx, sessionOccupancyKey(accountID), "session-a").Result()
	require.NoError(t, err)
	require.False(t, metadata)
}

func TestSessionOccupancyIncludesUnknownLegacySessionsAndRemovesExpiredMetadata(t *testing.T) {
	cache := NewSessionLimitCache(testRedis(t), 5)
	occupancy := cache.(service.SessionOccupancyCache)
	concrete := cache.(*sessionLimitCache)
	ctx := context.Background()
	const accountID int64 = 702

	now, err := concrete.rdb.Time(ctx).Result()
	require.NoError(t, err)
	require.NoError(t, concrete.rdb.ZAdd(ctx, sessionLimitKey(accountID),
		redis.Z{Score: float64(now.Unix()), Member: "legacy-session"},
		redis.Z{Score: float64(now.Add(-10 * time.Minute).Unix()), Member: "expired-session"},
	).Err())
	require.NoError(t, concrete.rdb.HSet(ctx, sessionOccupancyKey(accountID), "expired-session", "99").Err())

	owners, err := occupancy.GetSessionOccupants(ctx, accountID, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, owners, 1)
	require.Nil(t, owners[0].UserID)
	require.Equal(t, 1, owners[0].SessionCount)
	exists, err := concrete.rdb.Exists(ctx, sessionOccupancyKey(accountID)).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}

func TestSessionOccupancyCountCleanupRemovesExpiredOwnerMetadata(t *testing.T) {
	cache := NewSessionLimitCache(testRedis(t), 5)
	concrete := cache.(*sessionLimitCache)
	ctx := context.Background()
	const accountID int64 = 703
	now, err := concrete.rdb.Time(ctx).Result()
	require.NoError(t, err)
	require.NoError(t, concrete.rdb.ZAdd(ctx, sessionLimitKey(accountID), redis.Z{Score: float64(now.Add(-10 * time.Minute).Unix()), Member: "expired"}).Err())
	require.NoError(t, concrete.rdb.HSet(ctx, sessionOccupancyKey(accountID), "expired", "42").Err())

	count, err := cache.GetActiveSessionCount(ctx, accountID)
	require.NoError(t, err)
	require.Zero(t, count)
	exists, err := concrete.rdb.HExists(ctx, sessionOccupancyKey(accountID), "expired").Result()
	require.NoError(t, err)
	require.False(t, exists)
}

func TestSessionOccupancyDoesNotReassignExistingSessionOwner(t *testing.T) {
	cache := NewSessionLimitCache(testRedis(t), 5)
	occupancy := cache.(service.SessionOccupancyCache)
	ctx := context.Background()
	const accountID int64 = 704
	allowed, err := occupancy.RegisterSessionWithOwner(ctx, accountID, "shared-session", 2, 5*time.Minute, 11)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = occupancy.RegisterSessionWithOwner(ctx, accountID, "shared-session", 2, 5*time.Minute, 22)
	require.NoError(t, err)
	require.True(t, allowed)

	owners, err := occupancy.GetSessionOccupants(ctx, accountID, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, owners, 1)
	require.NotNil(t, owners[0].UserID)
	require.Equal(t, int64(11), *owners[0].UserID)
}
