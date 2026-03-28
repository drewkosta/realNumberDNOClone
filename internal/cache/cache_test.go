package cache

import (
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New[string](time.Minute, 100)
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Get(key1) = %q, %v; want value1, true", val, ok)
	}

	_, ok = c.Get("missing")
	if ok {
		t.Error("Get(missing) should return false")
	}
}

func TestTTLExpiration(t *testing.T) {
	c := New[string](50*time.Millisecond, 100)
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok || val != "value1" {
		t.Fatal("expected key1 to exist immediately after set")
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestDelete(t *testing.T) {
	c := New[string](time.Minute, 100)
	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}
}

func TestDeletePrefix(t *testing.T) {
	c := New[string](time.Minute, 100)
	c.Set("phone:5551234567:voice", "hit")
	c.Set("phone:5551234567:text", "miss")
	c.Set("phone:8001234567:voice", "hit")

	c.DeletePrefix("phone:5551234567:")

	_, ok1 := c.Get("phone:5551234567:voice")
	_, ok2 := c.Get("phone:5551234567:text")
	_, ok3 := c.Get("phone:8001234567:voice")

	if ok1 || ok2 {
		t.Error("expected phone:5551234567 keys to be deleted")
	}
	if !ok3 {
		t.Error("expected phone:8001234567 key to remain")
	}
}

func TestEvictionAtCapacity(t *testing.T) {
	c := New[int](time.Minute, 10)
	for i := 0; i < 20; i++ {
		c.Set(string(rune('a'+i)), i)
	}
	// Should not panic and should have fewer than 20 items
	c.mu.RLock()
	size := len(c.items)
	c.mu.RUnlock()
	if size > 10 {
		t.Errorf("cache size %d exceeds max 10", size)
	}
}
