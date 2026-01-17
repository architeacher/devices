package decorator_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/architeacher/devices/pkg/decorator"
)

type testQuery struct {
	ID string
}

type testResult struct {
	Value string
}

type mockCache struct {
	mu       sync.RWMutex
	data     map[string]testResult
	getCnt   int
	setCnt   int
	getErr   error
	setErr   error
	setDelay time.Duration
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]testResult),
	}
}

func (m *mockCache) Get(_ context.Context, query testQuery) (testResult, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getCnt++

	if m.getErr != nil {
		return testResult{}, false, m.getErr
	}

	result, ok := m.data[query.ID]

	return result, ok, nil
}

func (m *mockCache) Set(_ context.Context, query testQuery, result testResult, _ time.Duration) error {
	if m.setDelay > 0 {
		time.Sleep(m.setDelay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.setCnt++

	if m.setErr != nil {
		return m.setErr
	}

	m.data[query.ID] = result

	return nil
}

func (m *mockCache) GetCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getCnt
}

func (m *mockCache) SetCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.setCnt
}

type mockQueryHandler struct {
	mu        sync.Mutex
	callCount int
	result    testResult
	err       error
}

func (h *mockQueryHandler) Execute(_ context.Context, _ testQuery) (testResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.callCount++

	return h.result, h.err
}

func (h *mockQueryHandler) CallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.callCount
}

func TestQueryCachingDecorator_CacheHit(t *testing.T) {
	t.Parallel()

	cache := newMockCache()
	cache.data["test-id"] = testResult{Value: "cached-value"}

	handler := &mockQueryHandler{
		result: testResult{Value: "fresh-value"},
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		cache,
		decorator.CacheConfig{Enabled: true, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	result, err := decorated.Execute(context.Background(), query)

	require.NoError(t, err)
	require.Equal(t, "cached-value", result.Value)
	require.Equal(t, 0, handler.CallCount())
	require.Equal(t, 1, cache.GetCount())
}

func TestQueryCachingDecorator_CacheMiss(t *testing.T) {
	t.Parallel()

	cache := newMockCache()

	handler := &mockQueryHandler{
		result: testResult{Value: "fresh-value"},
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		cache,
		decorator.CacheConfig{Enabled: true, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	result, err := decorated.Execute(context.Background(), query)

	require.NoError(t, err)
	require.Equal(t, "fresh-value", result.Value)
	require.Equal(t, 1, handler.CallCount())
	require.Equal(t, 1, cache.GetCount())

	time.Sleep(10 * time.Millisecond)
	require.Equal(t, 1, cache.SetCount())
}

func TestQueryCachingDecorator_CacheDisabled(t *testing.T) {
	t.Parallel()

	cache := newMockCache()
	cache.data["test-id"] = testResult{Value: "cached-value"}

	handler := &mockQueryHandler{
		result: testResult{Value: "fresh-value"},
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		cache,
		decorator.CacheConfig{Enabled: false, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	result, err := decorated.Execute(context.Background(), query)

	require.NoError(t, err)
	require.Equal(t, "fresh-value", result.Value)
	require.Equal(t, 1, handler.CallCount())
	require.Equal(t, 0, cache.GetCount())
}

func TestQueryCachingDecorator_NilCache(t *testing.T) {
	t.Parallel()

	handler := &mockQueryHandler{
		result: testResult{Value: "fresh-value"},
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		nil,
		decorator.CacheConfig{Enabled: true, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	result, err := decorated.Execute(context.Background(), query)

	require.NoError(t, err)
	require.Equal(t, "fresh-value", result.Value)
	require.Equal(t, 1, handler.CallCount())
}

func TestQueryCachingDecorator_HandlerError(t *testing.T) {
	t.Parallel()

	cache := newMockCache()
	expectedErr := errors.New("handler error")

	handler := &mockQueryHandler{
		err: expectedErr,
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		cache,
		decorator.CacheConfig{Enabled: true, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	_, err := decorated.Execute(context.Background(), query)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	require.Equal(t, 1, handler.CallCount())

	time.Sleep(10 * time.Millisecond)
	require.Equal(t, 0, cache.SetCount())
}

func TestQueryCachingDecorator_CacheGetError(t *testing.T) {
	t.Parallel()

	cache := newMockCache()
	cache.getErr = errors.New("cache get error")

	handler := &mockQueryHandler{
		result: testResult{Value: "fresh-value"},
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		cache,
		decorator.CacheConfig{Enabled: true, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	result, err := decorated.Execute(context.Background(), query)

	require.NoError(t, err)
	require.Equal(t, "fresh-value", result.Value)
	require.Equal(t, 1, handler.CallCount())
}

func TestCacheStatus_ContextOperations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status decorator.CacheStatus
	}{
		{name: "HIT", status: decorator.CacheStatusHit},
		{name: "MISS", status: decorator.CacheStatusMiss},
		{name: "BYPASS", status: decorator.CacheStatusBypass},
		{name: "ERROR", status: decorator.CacheStatusError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := decorator.WithCacheStatus(context.Background(), tc.status)
			result := decorator.GetCacheStatus(ctx)

			require.Equal(t, tc.status, result)
		})
	}
}

func TestCacheStatus_DefaultValue(t *testing.T) {
	t.Parallel()

	status := decorator.GetCacheStatus(context.Background())
	require.Equal(t, decorator.CacheStatusBypass, status)
}

func TestQueryCachingDecorator_AsyncCacheSet(t *testing.T) {
	t.Parallel()

	cache := newMockCache()
	cache.setDelay = 50 * time.Millisecond

	handler := &mockQueryHandler{
		result: testResult{Value: "fresh-value"},
	}

	decorated := decorator.NewQueryCachingDecorator[testQuery, testResult](
		handler,
		cache,
		decorator.CacheConfig{Enabled: true, TTL: time.Minute},
	)

	query := testQuery{ID: "test-id"}
	start := time.Now()
	result, err := decorated.Execute(context.Background(), query)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, "fresh-value", result.Value)
	require.Less(t, elapsed, 30*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 1, cache.SetCount())
}
