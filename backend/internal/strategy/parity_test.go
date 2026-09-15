package strategy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Holdout window used by docs/donchian_backtest/streak_smooth (fresh $10k).
var (
	holdoutFrom = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	holdoutTo   = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
)

type researchTrade struct {
	Direction string
	Signal    int64
	Entry     int64
	Exit      int64
	Outcome   string
	EntryPx   float64
	Stop      float64
	ExitPx    float64
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// backend/internal/strategy → repo root
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func TestParityBTCHoldoutBaseline(t *testing.T) {
	assertParity(t, "btcusdt_1h", "docs/donchian_backtest/streak_smooth/btc_ho_baseline/trades.csv", false)
}

func TestParityBTCHoldoutLS5(t *testing.T) {
	assertParity(t, "btcusdt_1h", "docs/donchian_backtest/streak_smooth/btc_ho_ls5_cond_brk2.0/trades.csv", true)
}

func TestParityETHHoldoutBaseline(t *testing.T) {
	assertParity(t, "ethusdt_1h", "docs/donchian_backtest/streak_smooth/eth_ho_baseline/trades.csv", false)
}

func TestParityETHHoldoutLS5(t *testing.T) {
	assertParity(t, "ethusdt_1h", "docs/donchian_backtest/streak_smooth/eth_ho_ls5_cond_brk2.0/trades.csv", true)
}

func TestParityLS5TakesFewerTradesThanBaseline(t *testing.T) {
	root := repoRoot(t)
	bars, err := loadResearchBars(filepath.Join(root, "data", "btcusdt_1h"))
	if err != nil {
		t.Skip(err.Error())
	}
	base := DefaultConfig()
	base.LS5.Enabled = false
	ls5 := DefaultConfig()
	bt, _ := Replay(bars, base, 10_000, holdoutFrom, holdoutTo)
	lt, _ := Replay(bars, ls5, 10_000, holdoutFrom, holdoutTo)
	if len(lt) >= len(bt) {
		t.Fatalf("ls5 should skip some baseline entries: baseline=%d ls5=%d", len(bt), len(lt))
	}
}

func assertParity(t *testing.T, candleDir, csvRel string, ls5 bool) {
	t.Helper()
	root := repoRoot(t)
	bars, err := loadResearchBars(filepath.Join(root, "data", candleDir))
	if err != nil {
		t.Skip(err.Error())
	}
	want, err := loadResearchCSV(filepath.Join(root, csvRel))
	if err != nil {
		t.Fatal(err)
	}
	if len(want) < 100 {
		t.Fatalf("research CSV too short: %d rows", len(want))
	}
	cfg := DefaultConfig()
	cfg.FeeRate = 0.0005 // research CSVs were built with Binance VIP0; live/shadow books use 0
	if !ls5 {
		cfg.LS5.Enabled = false
	}
	got, _ := Replay(bars, cfg, 10_000, holdoutFrom, holdoutTo)
	closed := make([]Trade, 0, len(got))
	for _, tr := range got {
		if tr.Outcome == OutcomeOpen {
			continue
		}
		closed = append(closed, tr)
	}

	type key struct {
		entry int64
		dir   string
	}
	S := map[key]researchTrade{}
	L := map[key]Trade{}
	for _, r := range want {
		S[key{r.Entry, r.Direction}] = r
	}
	for _, tr := range closed {
		L[key{tr.EntryTime, string(tr.Direction)}] = tr
	}
	var matched, liveOnly, shadowOnly int
	var pxMiss int
	for k, r := range S {
		tr, ok := L[k]
		if !ok {
			shadowOnly++
			if shadowOnly <= 8 {
				t.Logf("go missed research %s entry=%s signal=%s", r.Direction, unixUTC(r.Entry), unixUTC(r.Signal))
			}
			continue
		}
		matched++
		if !priceClose(tr.EntryPrice, r.EntryPx) || !priceClose(tr.StopLoss, r.Stop) {
			pxMiss++
			if pxMiss <= 5 {
				t.Logf("price drift %s entry=%s go_entry=%.4f csv=%.4f go_stop=%.4f csv_stop=%.4f",
					r.Direction, unixUTC(r.Entry), tr.EntryPrice, r.EntryPx, tr.StopLoss, r.Stop)
			}
		}
	}
	for k, tr := range L {
		if _, ok := S[k]; !ok {
			liveOnly++
			if liveOnly <= 8 {
				t.Logf("go extra %s entry=%s signal=%s", tr.Direction, unixUTC(tr.EntryTime), unixUTC(tr.SignalTime))
			}
		}
	}
	union := matched + liveOnly + shadowOnly
	rate := 0.0
	if union > 0 {
		rate = float64(matched) / float64(union)
	}
	t.Logf("%s ls5=%v research=%d go=%d matched=%d extra=%d missed=%d rate=%.4f px_drift=%d bars=%d",
		candleDir, ls5, len(want), len(closed), matched, liveOnly, shadowOnly, rate, pxMiss, len(bars))
	if rate < 0.99 {
		t.Fatalf("parity %s < 99%% (matched=%d union=%d extra=%d missed=%d)",
			fmt.Sprintf("%.2f%%", rate*100), matched, union, liveOnly, shadowOnly)
	}
	if pxMiss > 0 && float64(pxMiss)/float64(matched) > 0.01 {
		t.Fatalf("entry/stop prices drifted on %d/%d matched trades", pxMiss, matched)
	}
}

func unixUTC(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04")
}

func priceClose(a, b float64) bool {
	if b == 0 {
		return math.Abs(a) < 1e-9
	}
	if math.Abs(a-b) <= 0.02 { // 2 cents — Binance tick
		return true
	}
	return math.Abs(a-b)/math.Abs(b) < 1e-6
}

func loadResearchBars(dir string) ([]Bar, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no research klines in %s (copy Binance Vision 1h JSON into data/btcusdt_1h and data/ethusdt_1h)", dir)
	}
	type kline struct {
		OpenTime int64   `json:"open_time"`
		Open     float64 `json:"open"`
		High     float64 `json:"high"`
		Low      float64 `json:"low"`
		Close    float64 `json:"close"`
		Volume   float64 `json:"volume"`
	}
	type fileJSON struct {
		Klines []kline `json:"klines"`
	}
	var bars []Bar
	for _, p := range matches {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var f fileJSON
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		for _, k := range f.Klines {
			if k.OpenTime <= 0 {
				continue
			}
			bars = append(bars, Bar{
				Time:   k.OpenTime / 1000,
				Open:   k.Open,
				High:   k.High,
				Low:    k.Low,
				Close:  k.Close,
				Volume: k.Volume,
			})
		}
	}
	if len(bars) < 200 {
		return nil, fmt.Errorf("%s: only %d bars", dir, len(bars))
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Time < bars[j].Time })
	out := bars[:0]
	var last int64 = -1
	for _, b := range bars {
		if b.Time == last {
			if len(out) > 0 {
				out[len(out)-1] = b
			}
			continue
		}
		out = append(out, b)
		last = b.Time
	}
	return out, nil
}

func loadResearchCSV(path string) ([]researchTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("%s: empty", path)
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.TrimSpace(h)] = i
	}
	need := []string{"direction", "signal_time", "entry_time", "exit_time", "outcome", "entry", "stop", "exit"}
	for _, n := range need {
		if _, ok := idx[n]; !ok {
			return nil, fmt.Errorf("%s: missing column %s", path, n)
		}
	}
	var out []researchTrade
	for _, row := range rows[1:] {
		if len(row) < len(rows[0]) {
			continue
		}
		sig, err := parseCSVTime(row[idx["signal_time"]])
		if err != nil {
			return nil, err
		}
		ent, err := parseCSVTime(row[idx["entry_time"]])
		if err != nil {
			return nil, err
		}
		ex, err := parseCSVTime(row[idx["exit_time"]])
		if err != nil {
			return nil, err
		}
		entryPx, _ := strconv.ParseFloat(row[idx["entry"]], 64)
		stop, _ := strconv.ParseFloat(row[idx["stop"]], 64)
		exitPx, _ := strconv.ParseFloat(row[idx["exit"]], 64)
		out = append(out, researchTrade{
			Direction: strings.ToUpper(strings.TrimSpace(row[idx["direction"]])),
			Signal:    sig,
			Entry:     ent,
			Exit:      ex,
			Outcome:   strings.TrimSpace(row[idx["outcome"]]),
			EntryPx:   entryPx,
			Stop:      stop,
			ExitPx:    exitPx,
		})
	}
	return out, nil
}

func parseCSVTime(s string) (int64, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if tm, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return tm.Unix(), nil
		}
	}
	return 0, fmt.Errorf("bad time %q", s)
}
