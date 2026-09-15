package plot

import (
	"bytes"
	"image/png"
	"testing"
)

func TestEquityPNGEmpty(t *testing.T) {
	b, err := EquityPNG(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

func TestEquityPNGSeries(t *testing.T) {
	live := []float64{100, 101, 99, 102, 103}
	sh := []float64{100, 100.5, 100.2, 101, 101.5}
	b, err := EquityPNG(live, sh)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 900 {
		t.Fatalf("width %d", img.Bounds().Dx())
	}
}
