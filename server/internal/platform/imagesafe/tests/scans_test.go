package imagesafe_test

import (
	"bytes"
	"context"
	"errors"
	"quizzivy/internal/platform/imagesafe"
	"testing"
	"time"
)

func TestAJpegOfMoreThanThirtyTwoScansIsRefusedAndOneOfThirtyTwoIsNot(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"1 scan", eobRunJPEG(400, 1), nil},
		{"10 scans, libjpeg's script for a colour photo", eobRunJPEG(400, 10), nil},
		{"32 scans", eobRunJPEG(400, 32), nil},
		{"33 scans", eobRunJPEG(400, 33), imagesafe.ErrDimensions},
		{"2048 x 2048 with 33 scans", eobRunJPEG(2048, 33), imagesafe.ErrDimensions},
		{"2048 x 2048 with 20000 scans", eobRunJPEG(2048, 20000), imagesafe.ErrDimensions},
	}
	for _, c := range cases {
		started := time.Now()
		_, err := square(t, c.data)
		if !errors.Is(err, c.want) {
			t.Errorf("%s answered %v, want %v", c.name, err, c.want)
		}
		if time.Since(started) > 5*time.Second {
			t.Errorf("%s took %v: the scans were decoded", c.name, time.Since(started))
		}
	}
}

func TestABaselineJpegWithManyScansIsBoundedByTheSameCap(t *testing.T) {
	if _, err := square(t, manyScanBaseline(400, 33)); !errors.Is(err, imagesafe.ErrDimensions) {
		t.Errorf("33 baseline scans answered %v, want ErrDimensions", err)
	}
	if _, err := square(t, manyScanBaseline(400, 3)); !errors.Is(err, imagesafe.ErrUnreadable) {
		t.Errorf("3 baseline scans answered %v, want them let through to a decode that fails (ErrUnreadable)", err)
	}
}

func TestTheScanCapIsTheCallersToSet(t *testing.T) {
	upload := eobRunJPEG(400, 12)
	for limit, want := range map[int]error{12: nil, 11: imagesafe.ErrDimensions, 0: imagesafe.ErrDimensions} {
		lim := avatarLimits
		lim.MaxScans = limit
		_, err := imagesafe.New(imagesafe.NewGate(1)).Square(context.Background(), bytes.NewReader(upload), lim)
		if !errors.Is(err, want) {
			t.Errorf("12 scans under a cap of %d answered %v, want %v", limit, err, want)
		}
	}
}

func TestAnEmptyScanFileIsRefusedWithoutWaitingForASlot(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	started := time.Now()
	_, err = imagesafe.New(gate).Square(ctx, bytes.NewReader(eobRunJPEG(2048, 20000)), avatarLimits)

	if !errors.Is(err, imagesafe.ErrDimensions) || time.Since(started) > 5*time.Second {
		t.Errorf("answered %v after %v, want ErrDimensions at once while every slot is held", err, time.Since(started))
	}
}
