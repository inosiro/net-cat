package chat_test

import (
	"sync"
	"testing"

	"netcat/internal/chat"
)

func TestSafeClose(t *testing.T) {
	t.Run("Single Close", func(t *testing.T) {
		c := &chat.Client{}
		if !c.SafeClose() {
			t.Errorf("Expected first close to return true")
		}
	})

	t.Run("Double Close", func(t *testing.T) {
		c := &chat.Client{}
		c.SafeClose()
		if c.SafeClose() {
			t.Errorf("Expected second close to return false")
		}
	})

	t.Run("Concurrent Close", func(t *testing.T) {
		c := &chat.Client{}
		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if c.SafeClose() {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful close, got %d", successCount)
		}
	})
}
