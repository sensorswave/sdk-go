package sensorswave

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestEventJSONSerialization(t *testing.T) {
	// Create and normalize event
	event := NewEvent("anon-123", "user-456", "TestEvent").
		WithProperties(NewProperties().Set("test_key", "test_value"))

	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	// Serialize to JSON
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	// Deserialize back
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	// Verify properties
	if decoded.Properties[PspLib] != sdkType {
		t.Errorf("expected $lib in decoded event to be '%s', got '%v'", sdkType, decoded.Properties[PspLib])
	}
	if decoded.Properties[PspLibVersion] != version {
		t.Errorf("expected $lib_version in decoded event to be '%s', got '%v'", version, decoded.Properties[PspLibVersion])
	}
}

func TestTrackDropsWhenMessageChannelIsFull(t *testing.T) {
	c := &client{
		cfg:     &Config{Logger: &noopLogger{}},
		msgchan: make(chan []byte, 1),
	}
	c.msgchan <- []byte(`{"event":"already-queued"}`)

	done := make(chan error, 1)
	go func() {
		done <- c.Track(NewEvent("", "user-1", "DroppedWhenFull"))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Track should drop full-queue events without surfacing an error, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Track blocked waiting for message channel capacity")
	}
}

func TestTrackAndCloseConcurrentDoesNotPanic(t *testing.T) {
	c, err := NewWithConfig(
		Endpoint("https://collector.example.com"),
		SourceToken("token"),
		Config{Logger: &noopLogger{}, FlushInterval: time.Hour},
	)
	if err != nil {
		t.Fatalf("NewWithConfig error: %v", err)
	}

	var wg sync.WaitGroup
	panicCh := make(chan any, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				panicCh <- r
			}
		}()
		_ = c.Track(NewEvent("", "user-1", "ConcurrentClose"))
	}()
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				panicCh <- r
			}
		}()
		_ = c.Close()
	}()
	wg.Wait()

	select {
	case r := <-panicCh:
		t.Fatalf("Track/Close concurrency must not panic: %v", r)
	default:
	}
}

func TestSendCompressesBatchAboveGzipThreshold(t *testing.T) {
	var gotEncoding string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if gotEncoding == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Errorf("gzip reader error: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer reader.Close()
			body, err = io.ReadAll(reader)
			if err != nil {
				t.Errorf("gzip read error: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c, err := NewWithConfig(
		Endpoint(server.URL),
		SourceToken("token"),
		Config{
			Logger:             &noopLogger{},
			FlushInterval:      time.Hour,
			HTTPConcurrency:    1,
			HTTPTimeout:        time.Second,
			GzipThresholdBytes: 1,
		},
	)
	if err != nil {
		t.Fatalf("NewWithConfig error: %v", err)
	}

	if err := c.Track(NewEvent("", "user-1", "CompressedEvent")); err != nil {
		t.Fatalf("Track error: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if gotEncoding != "gzip" {
		t.Fatalf("expected gzip content encoding, got %q", gotEncoding)
	}
	var events []Event
	if err := json.Unmarshal(gotBody, &events); err != nil {
		t.Fatalf("decode request body error: %v body=%s", err, string(gotBody))
	}
	if len(events) != 1 || events[0].Event != "CompressedEvent" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestNativePropertyTimeAlwaysNormalizedToUTCRegardlessOfZone(t *testing.T) {
	// Same absolute instant expressed in three different zones.
	// All three must produce the same ISO8601 UTC string.
	shanghai := time.FixedZone("CST", 8*3600)
	newYork := time.FixedZone("EDT", -4*3600)
	utcTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)
	shanghaiTime := time.Date(2026, 4, 23, 16, 15, 30, 123000000, shanghai)
	newYorkTime := time.Date(2026, 4, 23, 4, 15, 30, 123000000, newYork)

	for name, pt := range map[string]time.Time{"utc": utcTime, "shanghai": shanghaiTime, "newYork": newYorkTime} {
		event := NewEvent("anon-123", "user-456", "ZoneProbe").
			WithTime(1776932130123).
			WithProperties(NewProperties().Set("t", pt))
		if err := event.Normalize(); err != nil {
			t.Fatalf("%s: normalize error: %v", name, err)
		}
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("%s: json marshal error: %v", name, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s: json unmarshal error: %v", name, err)
		}
		got := decoded["properties"].(map[string]any)["t"]
		if got != "2026-04-23T08:15:30.123Z" {
			t.Fatalf("%s: expected UTC-normalized ISO8601 string, got %v", name, got)
		}
	}
}

func TestPropertySettersPreserveNativeTypesUntilNormalize(t *testing.T) {
	propertyTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)

	props := NewProperties().Set("t", propertyTime)
	if _, ok := props["t"].(time.Time); !ok {
		t.Fatalf("Properties.Set must store raw time.Time until Normalize; got %T", props["t"])
	}

	up := NewUserPropertyOpts().
		Set("registered_at", propertyTime).
		SetOnce("first_seen_at", propertyTime).
		Append("milestones", []any{propertyTime, propertyTime}).
		Union("tags", []any{propertyTime, propertyTime})

	if _, ok := up["$set"].(map[string]any)["registered_at"].(time.Time); !ok {
		t.Fatalf("UserPropertyOpts.Set must store raw time.Time; got %T", up["$set"].(map[string]any)["registered_at"])
	}
	if _, ok := up["$set_once"].(map[string]any)["first_seen_at"].(time.Time); !ok {
		t.Fatalf("UserPropertyOpts.SetOnce must store raw time.Time; got %T", up["$set_once"].(map[string]any)["first_seen_at"])
	}
	appendList := up["$append"].(map[string]any)["milestones"].([]any)
	if len(appendList) != 2 {
		t.Fatalf("Append must not dedupe nor transform; got %d elements", len(appendList))
	}
	if _, ok := appendList[0].(time.Time); !ok {
		t.Fatalf("Append must store raw time.Time; got %T", appendList[0])
	}
	unionList := up["$union"].(map[string]any)["tags"].([]any)
	if len(unionList) != 2 {
		t.Fatalf("Union setter must be append-only (dedupe happens in Normalize); got %d elements", len(unionList))
	}

	// After Normalize, values are strings and $union is deduped.
	event := NewEvent("", "user-456", PseUserSet).
		WithTime(1776932130123).
		WithUserPropertyOpts(up).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnion))
	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	normalizedUnion := event.UserProperties["$union"].(map[string]any)["tags"].([]any)
	if len(normalizedUnion) != 1 {
		t.Fatalf("Normalize must dedupe $union to 1 element; got %d", len(normalizedUnion))
	}
	if normalizedUnion[0] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("Normalize must format to ISO8601 UTC; got %v", normalizedUnion[0])
	}
	normalizedAppend := event.UserProperties["$append"].(map[string]any)["milestones"].([]any)
	if len(normalizedAppend) != 2 {
		t.Fatalf("Normalize must NOT dedupe $append; got %d elements", len(normalizedAppend))
	}
}
