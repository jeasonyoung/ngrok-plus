package cache

import (
	"container/list"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LRUCache LRU缓存
type lruCache struct {
	mu sync.RWMutex
	//list & table of *entry objects
	list  *list.List
	table map[string]*list.Element
	//Our current size,in bytes,Obviously a gross simplification and low-gate approximation.
	size uint64
	//How many bytes we are limiting the cache to.
	capacity uint64
}

type entry struct {
	key          string
	value        Value
	size         int
	timeAccessed time.Time
}

// NewLRUCache 新建缓存实例
func NewLRUCache(capacity uint64) Cache {
	return &lruCache{
		list:     list.New(),
		table:    make(map[string]*list.Element),
		capacity: capacity,
	}
}

// LoadItems 加载缓存项
func (lru *lruCache) LoadItems(r io.Reader) error {
	items := make([]Item, 0)
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&items); err != nil {
		return err
	}
	lru.mu.Lock()
	defer lru.mu.Unlock()
	for _, item := range items {
		//XXX: copied from Set()
		if elem := lru.table[item.Key]; elem != nil {
			lru.updateInPlace(elem, item.Value)
		} else {
			lru.addNew(item.Key, item.Value)
		}
	}
	return nil
}

// LoadItemsFromFile 从文件加载缓存项
func (lru *lruCache) LoadItemsFromFile(path string) error {
	if rd, err := os.Open(path); err != nil {
		return err
	} else {
		defer func(rd *os.File) {
			_ = rd.Close()
		}(rd)
		return lru.LoadItems(rd)
	}
}

// SaveItems 保存缓存项
func (lru *lruCache) SaveItems(w io.Writer) error {
	items := lru.Items()
	encoder := gob.NewEncoder(w)
	return encoder.Encode(items)
}

// SaveItemsToFile 保存缓存项到文件
func (lru *lruCache) SaveItemsToFile(path string) error {
	if wr, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err != nil {
		return err
	} else {
		defer func(wr *os.File) {
			_ = wr.Close()
		}(wr)
		return lru.SaveItems(wr)
	}
}

// SetCapacity 设置容量
func (lru *lruCache) SetCapacity(capacity uint64) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	lru.capacity = capacity
	lru.checkCapacity()
}

// Keys 缓存键集合
func (lru *lruCache) Keys() []string {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	//
	keys := make([]string, 0, lru.list.Len())
	for e := lru.list.Front(); e != nil; e = e.Next() {
		keys = append(keys, e.Value.(*entry).key)
	}
	return keys
}

// Items 缓存项集合
func (lru *lruCache) Items() []Item {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	//
	items := make([]Item, 0, lru.list.Len())
	for e := lru.list.Front(); e != nil; e = e.Next() {
		v := e.Value.(*entry)
		items = append(items, Item{Key: v.key, Value: v.value})
	}
	return items
}

// Get 获取缓存值
func (lru *lruCache) Get(key string) (v Value, ok bool) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	//
	elem := lru.table[key]
	if elem == nil {
		return nil, false
	}
	lru.moveToFront(elem)
	return elem.Value.(*entry).value, true
}

// Set 设置缓存值
func (lru *lruCache) Set(key string, value Value) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if elem := lru.table[key]; elem != nil {
		lru.updateInPlace(elem, value)
	} else {
		lru.addNew(key, value)
	}
	return
}

// SetIfAbsent 设置缓存值
func (lru *lruCache) SetIfAbsent(key string, value Value) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if elem := lru.table[key]; elem != nil {
		lru.moveToFront(elem)
	} else {
		lru.addNew(key, value)
	}
}

// Delete 删除缓存
func (lru *lruCache) Delete(key string) bool {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	elem := lru.table[key]
	if elem == nil {
		return false
	}
	lru.list.Remove(elem)
	delete(lru.table, key)

	lru.size -= uint64(elem.Value.(*entry).size)
	return true
}

// Clear 清空缓存
func (lru *lruCache) Clear() {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	lru.list.Init()
	lru.table = make(map[string]*list.Element)
	lru.size = 0
}

// Stats 统计信息
func (lru *lruCache) Stats() (length, size, capacity uint64, oldest time.Time) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if lastElem := lru.list.Back(); lastElem != nil {
		oldest = lastElem.Value.(*entry).timeAccessed
	}
	return uint64(lru.list.Len()), lru.size, lru.capacity, oldest
}

// StatsJSON 统计信息JSON
func (lru *lruCache) StatsJSON() string {
	if lru == nil {
		return "{}"
	}
	l, s, c, o := lru.Stats()
	return fmt.Sprintf("{\"Length\": %v, \"Size\": %v, \"Capacity\": %v, \"OldestAccess\":\"%v\"}", l, s, c, o)
}

func (lru *lruCache) updateInPlace(elem *list.Element, value Value) {
	valueSize := value.Size()
	sizeDiff := valueSize - elem.Value.(*entry).size
	elem.Value.(*entry).value = value
	elem.Value.(*entry).size = valueSize
	lru.size += uint64(sizeDiff)
	lru.moveToFront(elem)
	lru.checkCapacity()
}

func (lru *lruCache) moveToFront(elem *list.Element) {
	lru.list.MoveToFront(elem)
	elem.Value.(*entry).timeAccessed = time.Now()
}

func (lru *lruCache) addNew(key string, value Value) {
	newEntry := &entry{
		key:          key,
		value:        value,
		size:         value.Size(),
		timeAccessed: time.Now(),
	}
	elem := lru.list.PushFront(newEntry)
	lru.table[key] = elem
	lru.size += uint64(newEntry.size)
	lru.checkCapacity()
}

func (lru *lruCache) checkCapacity() {
	// Partially duplicated from Delete
	for lru.size > lru.capacity {
		delElem := lru.list.Back()
		delValue := delElem.Value.(*entry)
		lru.list.Remove(delElem)
		delete(lru.table, delValue.key)
		lru.size -= uint64(delValue.size)
	}
}
