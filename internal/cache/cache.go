package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yourusername/sam-gov-monitor/internal/samgov"
)

// CacheEntry represents a cached response with metadata
type CacheEntry struct {
	Data      *samgov.SearchResponse `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	TTL       time.Duration          `json:"ttl"`
	QueryHash string                 `json:"query_hash"`
}

// Cache provides a simple file-based caching mechanism for API responses
type Cache struct {
	mu        sync.RWMutex
	cacheDir  string
	defaultTTL time.Duration
	maxSize   int64 // Maximum cache size in bytes
	verbose   bool
	entries   map[string]*CacheEntry // In-memory index
}

// NewCache creates a new cache instance
func NewCache(cacheDir string, defaultTTL time.Duration, verbose bool) (*Cache, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	cache := &Cache{
		cacheDir:   cacheDir,
		defaultTTL: defaultTTL,
		maxSize:    100 * 1024 * 1024, // 100MB default
		verbose:    verbose,
		entries:    make(map[string]*CacheEntry),
	}

	// Load existing cache entries
	if err := cache.loadIndex(); err != nil && verbose {
		fmt.Printf("Warning: Could not load cache index: %v\n", err)
	}

	return cache, nil
}

// Get retrieves a response from cache if it exists and is not expired
func (c *Cache) Get(params map[string]string) (*samgov.SearchResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.generateKey(params)
	entry, exists := c.entries[key]
	
	if !exists {
		if c.verbose {
			fmt.Printf("Cache miss for key: %s\n", key[:8])
		}
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > entry.TTL {
		if c.verbose {
			fmt.Printf("Cache expired for key: %s (age: %v, TTL: %v)\n", 
				key[:8], time.Since(entry.Timestamp), entry.TTL)
		}
		// Mark for cleanup but don't remove immediately (defer to cleanup process)
		return nil, false
	}

	// Load data from file
	data, err := c.loadFromFile(key)
	if err != nil {
		if c.verbose {
			fmt.Printf("Cache file read error for key %s: %v\n", key[:8], err)
		}
		return nil, false
	}

	if c.verbose {
		fmt.Printf("Cache hit for key: %s (age: %v)\n", 
			key[:8], time.Since(entry.Timestamp))
	}

	return data, true
}

// Set stores a response in the cache
func (c *Cache) Set(params map[string]string, response *samgov.SearchResponse) error {
	return c.SetWithTTL(params, response, c.defaultTTL)
}

// SetWithTTL stores a response in the cache with custom TTL
func (c *Cache) SetWithTTL(params map[string]string, response *samgov.SearchResponse, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.generateKey(params)
	
	// Save to file
	if err := c.saveToFile(key, response); err != nil {
		return fmt.Errorf("saving to cache file: %w", err)
	}

	// Update in-memory index
	entry := &CacheEntry{
		Data:      response,
		Timestamp: time.Now(),
		TTL:       ttl,
		QueryHash: key,
	}
	
	c.entries[key] = entry

	if c.verbose {
		fmt.Printf("Cached response for key: %s (TTL: %v, %d opportunities)\n", 
			key[:8], ttl, len(response.OpportunitiesData))
	}

	// Trigger cleanup if cache is getting large
	if len(c.entries) > 1000 {
		go c.cleanup()
	}

	return nil
}

// Delete removes an entry from the cache
func (c *Cache) Delete(params map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.generateKey(params)
	
	// Remove from memory
	delete(c.entries, key)
	
	// Remove file
	filePath := c.getFilePath(key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing cache file: %w", err)
	}

	if c.verbose {
		fmt.Printf("Deleted cache entry: %s\n", key[:8])
	}

	return nil
}

// Clear removes all entries from the cache
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all files
	for key := range c.entries {
		filePath := c.getFilePath(key)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing cache file %s: %w", key[:8], err)
		}
	}

	// Clear memory
	c.entries = make(map[string]*CacheEntry)

	if c.verbose {
		fmt.Printf("Cache cleared\n")
	}

	return nil
}

// GetStats returns cache statistics
func (c *Cache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_entries"] = len(c.entries)
	stats["cache_dir"] = c.cacheDir
	stats["default_ttl"] = c.defaultTTL.String()

	// Count expired entries
	expired := 0
	oldest := time.Now()
	newest := time.Time{}
	
	for _, entry := range c.entries {
		if time.Since(entry.Timestamp) > entry.TTL {
			expired++
		}
		if entry.Timestamp.Before(oldest) {
			oldest = entry.Timestamp
		}
		if entry.Timestamp.After(newest) {
			newest = entry.Timestamp
		}
	}

	stats["expired_entries"] = expired
	if len(c.entries) > 0 {
		stats["oldest_entry"] = oldest
		stats["newest_entry"] = newest
	}

	// Get cache directory size
	if size, err := c.getCacheSize(); err == nil {
		stats["cache_size_bytes"] = size
		stats["cache_size_mb"] = float64(size) / (1024 * 1024)
	}

	return stats
}

// Cleanup removes expired entries and manages cache size
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	now := time.Now()

	// Remove expired entries
	for key, entry := range c.entries {
		if now.Sub(entry.Timestamp) > entry.TTL {
			filePath := c.getFilePath(key)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				if c.verbose {
					fmt.Printf("Error removing expired cache file %s: %v\n", key[:8], err)
				}
				continue
			}
			delete(c.entries, key)
			removed++
		}
	}

	if c.verbose && removed > 0 {
		fmt.Printf("Cache cleanup: removed %d expired entries\n", removed)
	}

	// Check cache size and remove oldest entries if necessary
	if size, err := c.getCacheSize(); err == nil && size > c.maxSize {
		c.evictOldest(int(size-c.maxSize) / 10240) // Remove ~10KB worth of entries
	}
}

// generateKey creates a unique key for the given parameters
func (c *Cache) generateKey(params map[string]string) string {
	// Create a consistent string from parameters
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	
	// Sort keys for consistency
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	var paramStr string
	for _, k := range keys {
		paramStr += fmt.Sprintf("%s=%s&", k, params[k])
	}

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(paramStr))
	return fmt.Sprintf("%x", hash)
}

// getFilePath returns the file path for a cache key
func (c *Cache) getFilePath(key string) string {
	return filepath.Join(c.cacheDir, key+".json")
}

// saveToFile saves a response to a cache file
func (c *Cache) saveToFile(key string, response *samgov.SearchResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}

	filePath := c.getFilePath(key)
	return os.WriteFile(filePath, data, 0644)
}

// loadFromFile loads a response from a cache file
func (c *Cache) loadFromFile(key string) (*samgov.SearchResponse, error) {
	filePath := c.getFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	var response samgov.SearchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return &response, nil
}

// loadIndex loads the cache index from existing files
func (c *Cache) loadIndex() error {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return fmt.Errorf("reading cache directory: %w", err)
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			key := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Create basic cache entry (data will be loaded on demand)
			cacheEntry := &CacheEntry{
				Timestamp: info.ModTime(),
				TTL:       c.defaultTTL,
				QueryHash: key,
			}
			
			c.entries[key] = cacheEntry
			loaded++
		}
	}

	if c.verbose && loaded > 0 {
		fmt.Printf("Loaded %d cache entries from disk\n", loaded)
	}

	return nil
}

// getCacheSize calculates the total size of cache files
func (c *Cache) getCacheSize() (int64, error) {
	var totalSize int64
	
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			totalSize += info.Size()
		}
	}

	return totalSize, nil
}

// evictOldest removes the oldest cache entries
func (c *Cache) evictOldest(count int) {
	if count <= 0 || len(c.entries) == 0 {
		return
	}

	// Sort entries by timestamp
	type entryWithKey struct {
		key   string
		entry *CacheEntry
	}

	entries := make([]entryWithKey, 0, len(c.entries))
	for k, v := range c.entries {
		entries = append(entries, entryWithKey{k, v})
	}

	// Simple bubble sort by timestamp (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].entry.Timestamp.After(entries[j].entry.Timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove oldest entries
	removed := 0
	for i := 0; i < count && i < len(entries); i++ {
		key := entries[i].key
		filePath := c.getFilePath(key)
		
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			continue
		}
		
		delete(c.entries, key)
		removed++
	}

	if c.verbose && removed > 0 {
		fmt.Printf("Cache eviction: removed %d oldest entries\n", removed)
	}
}

// StartCleanupTimer starts a periodic cleanup process
func (c *Cache) StartCleanupTimer(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.cleanup()
		}
	}()
}