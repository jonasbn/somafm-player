package spectrum

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func sineWavePCM(freq float64, sampleRate, samples int) []byte {
	pcm := make([]byte, samples*4)
	for i := 0; i < samples; i++ {
		v := math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
		s := int16(v * 20000)
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(s))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(s))
	}
	return pcm
}

func waitForNonZeroBands(a *Analyzer, timeout time.Duration) []float64 {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		bands := a.Bands()
		for _, v := range bands {
			if v != 0 {
				return bands
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	return a.Bands()
}

func TestAnalyzer_BandsInitiallyAllZero(t *testing.T) {
	a := New(44100)
	defer a.Close()

	bands := a.Bands()
	if len(bands) != analysisBands {
		t.Fatalf("len(Bands()) = %d, want %d", len(bands), analysisBands)
	}
	for i, v := range bands {
		if v != 0 {
			t.Errorf("bands[%d] = %v, want 0 before any Feed", i, v)
		}
	}
}

func TestAnalyzer_BandsReturnsDefensiveCopy(t *testing.T) {
	a := New(44100)
	defer a.Close()

	got := a.Bands()
	got[0] = 99

	again := a.Bands()
	if again[0] == 99 {
		t.Fatal("mutating a returned Bands() slice affected the analyzer's internal state")
	}
}

func TestAnalyzer_FeedSineWaveProducesPeakInExpectedBand(t *testing.T) {
	a := New(44100)
	defer a.Close()

	pcm := sineWavePCM(440.0, 44100, windowSize)
	a.Feed(pcm)

	bands := waitForNonZeroBands(a, 2*time.Second)

	peak := 0
	for i, v := range bands {
		if v > bands[peak] {
			peak = i
		}
	}
	if peak != 14 {
		t.Fatalf("peak band = %d, want 14 (~440Hz bucket at 44.1kHz/2048-sample window); bands=%v", peak, bands)
	}
}

func TestAnalyzer_CloseThenFeedDoesNotPanic(t *testing.T) {
	a := New(44100)
	a.Close()
	a.Feed(make([]byte, 16))
}
