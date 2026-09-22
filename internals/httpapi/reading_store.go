package httpapi

import "sync"

type readingStore struct {
	mu       sync.RWMutex
	readings []Reading
}

func newReadingStore() *readingStore {
	return &readingStore{}
}

func (s *readingStore) Save(reading Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.readings = append(s.readings, reading)
}

func (s *readingStore) Latest(deviceID string) (Reading, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := len(s.readings) - 1; i >= 0; i-- {
		if s.readings[i].DeviceID == deviceID {
			return s.readings[i], true
		}
	}

	return Reading{}, false
}
