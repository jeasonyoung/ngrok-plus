package cache

import (
	"io"
	"time"
)

// Value Values that go into lruCache need to satisfy this interface.
type Value interface {
	Size() int
}

// Item 缓存项
type Item struct {
	Key   string
	Value Value
}

// Cache 缓存接口
type Cache interface {
	// LoadItems 加载缓存项
	LoadItems(r io.Reader) error
	// LoadItemsFromFile 从文件加载缓存项
	LoadItemsFromFile(path string) error
	// SaveItems 保存缓存项
	SaveItems(w io.Writer) error
	// SaveItemsToFile 保存缓存项到文件
	SaveItemsToFile(path string) error
	// SetCapacity 设置容量
	SetCapacity(capacity uint64)
	// Keys 缓存键集合
	Keys() []string
	// Items 缓存项集合
	Items() []Item
	// Get 获取缓存值
	Get(key string) (v Value, ok bool)
	// Set 设置缓存值
	Set(key string, value Value)
	// SetIfAbsent 设置缓存值
	SetIfAbsent(key string, value Value)
	// Delete 删除缓存
	Delete(key string) bool
	// Clear 清空缓存
	Clear()
	// Stats 统计信息
	Stats() (length, size, capacity uint64, oldest time.Time)
	// StatsJSON 统计信息JSON
	StatsJSON() string
}
