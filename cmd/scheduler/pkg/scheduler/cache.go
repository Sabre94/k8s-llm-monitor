package scheduler

import (
	"sync"
	"time"
)

// ScheduleCache 调度结果缓存
type ScheduleCache struct {
	cache     map[string]*CacheEntry
	ttl       time.Duration
	maxSize   int
	hitCount  int64
	totalCount int64
	mu        sync.RWMutex
}

// CacheEntry 缓存条目
type CacheEntry struct {
	data      *ScheduleResult
	timestamp time.Time
	ttl       time.Duration
}

// NewScheduleCache 创建调度缓存
func NewScheduleCache(ttl time.Duration, maxSize int) *ScheduleCache {
	return &ScheduleCache{
		cache:   make(map[string]*CacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get 获取缓存项
func (c *ScheduleCache) Get(key string) *ScheduleResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.totalCount++

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	// 检查是否过期
	if time.Since(entry.timestamp) > entry.ttl {
		delete(c.cache, key)
		return nil
	}

	c.hitCount++
	return entry.data
}

// Set 设置缓存项
func (c *ScheduleCache) Set(key string, data *ScheduleResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的项
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	c.cache[key] = &CacheEntry{
		data:      data,
		timestamp: time.Now(),
		ttl:       c.ttl,
	}
}

// Delete 删除缓存项
func (c *ScheduleCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

// Clear 清空缓存
func (c *ScheduleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)
	c.hitCount = 0
	c.totalCount = 0
}

// CleanExpired 清理过期项
func (c *ScheduleCache) CleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.cache, key)
		}
	}
}

// Size 返回缓存大小
func (c *ScheduleCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// GetHitRate 返回缓存命中率
func (c *ScheduleCache) GetHitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.totalCount == 0 {
		return 0.0
	}

	return float64(c.hitCount) / float64(c.totalCount)
}

// evictOldest 驱逐最旧的缓存项
func (c *ScheduleCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, entry := range c.cache {
		if entry.timestamp.Before(oldestTime) {
			oldestTime = entry.timestamp
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// DataCache 数据缓存（已在crd_provider.go中定义，这里保持一致性）
// 注意：这个类型在crd_provider.go中已定义，这里仅作说明