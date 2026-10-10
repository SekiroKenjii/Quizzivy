package imagesafe_test

import (
	"bytes"
	"context"
	"errors"
	"quizzivy/internal/platform/imagesafe"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAGateNeverLetsMoreCallersInThanItHasSlots(t *testing.T) {
	gate := imagesafe.NewGate(2)
	var inside, peak atomic.Int32
	var wg sync.WaitGroup
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := gate.Acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			defer release()
			now := inside.Add(1)
			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}
			time.Sleep(3 * time.Millisecond)
			inside.Add(-1)
		}()
	}
	wg.Wait()

	if got := peak.Load(); got != 2 {
		t.Errorf("at most %d callers were inside at once, want exactly 2", got)
	}
}

func TestReleasingTwiceGivesBackOnlyOneSlot(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
	again, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer again()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := gate.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a second caller got in (%v) although one slot is held", err)
	}
}

func TestACallerWhoseRequestHasEndedDoesNotTakeASlot(t *testing.T) {
	gate := imagesafe.NewGate(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := gate.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("answered %v, want the context's error although a slot is free", err)
	}
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatalf("the slot was taken: %v", err)
	}
	release()
}

func TestADecodeWaitsForASlotOnlyAfterTheWholeFileIsRead(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	upload := encodePNG(t, solid(300, 300, red))
	body := &countingReader{r: bytes.NewReader(upload)}
	processor := imagesafe.New(gate)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = processor.Square(ctx, body, avatarLimits)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("answered %v, want the wait for a slot to end with the context", err)
	}
	if body.read != len(upload) {
		t.Errorf("read %d of %d bytes before waiting: a slow upload must not hold a slot", body.read, len(upload))
	}

	release()
	if _, err := processor.Square(context.Background(), bytes.NewReader(upload), avatarLimits); err != nil {
		t.Errorf("after the slot was given back, answered %v", err)
	}
}
