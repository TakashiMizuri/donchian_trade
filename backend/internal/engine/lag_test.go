package engine

import "testing"

func TestSummarizeLagHealthy(t *testing.T) {
	samples := []LagSample{
		{Symbol: "BTC", DetectLagMs: 800, Source: "boundary"},
		{Symbol: "ETH", DetectLagMs: 900, Source: "boundary"},
		{Symbol: "BTC", DetectLagMs: 1200, Source: "ws"},
	}
	st := summarizeLag(samples)
	if !st.Healthy || st.MedianMs > 1200 || st.Alert != "" {
		t.Fatalf("%+v", st)
	}
	if st.BySource["boundary"] != 2 || st.BySource["ws"] != 1 {
		t.Fatalf("sources %+v", st.BySource)
	}
}

func TestSummarizeLagCritical(t *testing.T) {
	samples := []LagSample{
		{Symbol: "BTC", DetectLagMs: 180000, Source: "reconcile"},
		{Symbol: "BTC", DetectLagMs: 200000, Source: "reconcile"},
		{Symbol: "ETH", DetectLagMs: 210000, Source: "reconcile"},
	}
	st := summarizeLag(samples)
	if st.Healthy || st.Gt60sPct < 99 || st.Alert == "" {
		t.Fatalf("%+v", st)
	}
}
