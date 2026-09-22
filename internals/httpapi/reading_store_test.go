package httpapi

import "testing"

func TestReadingStoreReturnsLatestReadingForDevice(t *testing.T) {
	store := newReadingStore()

	store.Save(Reading{
		DeviceID:    "plant-8",
		Temperature: 20,
	})

	store.Save(Reading{
		DeviceID:    "plant-9",
		Temperature: 18,
	})

	expected := Reading{
		DeviceID:    "plant-8",
		Temperature: 25,
	}

	store.Save(expected)

	got, found := store.Latest("plant-8")

	if !found {
		t.Fatal("expected a reading for plant-8")
	}

	if got != expected {
		t.Errorf("expected %#v, got %#v", expected, got)
	}
}

func TestReadingStoreReportsMissingDevice(t *testing.T) {
	store := newReadingStore()

	_, found := store.Latest("unknown-device")

	if found {
		t.Error("expected no reading for unknown-device")
	}
}
