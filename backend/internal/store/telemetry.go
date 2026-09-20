package store

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) migrateTelemetry() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bar_logs (
  symbol TEXT NOT NULL,
  bar_time INTEGER NOT NULL,
  atr REAL, upper_n REAL, lower_n REAL, close REAL,
  want_baseline TEXT, want_ls5 TEXT,
  ls5_pause_active INTEGER, ls5_resume_ready INTEGER,
  live_desired TEXT, live_actual TEXT,
  shadow_ls5_pos TEXT, shadow_baseline_pos TEXT,
  PRIMARY KEY (symbol, bar_time)
);`,
		`CREATE TABLE IF NOT EXISTS equity_curve (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  reason TEXT NOT NULL,
  equity_live REAL, equity_live_ex_funding REAL,
  upnl_live REAL, wallet_cash REAL,
  equity_shadow_ls5 REAL, equity_shadow_baseline REAL,
  gap_vs_ls5 REAL, cum_funding REAL,
  cum_fees_live REAL, cum_fees_shadow_ls5 REAL,
  match_rate_ltd REAL, side_slip_median_ltd_bps REAL,
  verdict TEXT
);`,
		`CREATE INDEX IF NOT EXISTS idx_equity_curve_ts ON equity_curve(ts);`,
		`CREATE TABLE IF NOT EXISTS mismatches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  kind TEXT NOT NULL,
  symbol TEXT, direction TEXT, signal_time INTEGER,
  note TEXT
);`,
		`CREATE TABLE IF NOT EXISTS daily_reports (
  date_utc TEXT PRIMARY KEY,
  ts INTEGER NOT NULL,
  body TEXT NOT NULL,
  verdict TEXT
);`,
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return err
		}
	}
	alters := []string{
		`ALTER TABLE trades ADD COLUMN entry_px_shadow REAL`,
		`ALTER TABLE trades ADD COLUMN entry_px_live REAL`,
		`ALTER TABLE trades ADD COLUMN exit_px_shadow REAL`,
		`ALTER TABLE trades ADD COLUMN exit_px_live REAL`,
		`ALTER TABLE trades ADD COLUMN atr_signal REAL`,
		`ALTER TABLE trades ADD COLUMN risk_usd REAL`,
		`ALTER TABLE trades ADD COLUMN skipped_reason TEXT`,
		`ALTER TABLE trades ADD COLUMN live_desync INTEGER DEFAULT 0`,
	}
	for _, q := range alters {
		_, _ = s.DB.Exec(q) // already exists after first run
	}
	return nil
}

type BarLog struct {
	Symbol, WantBaseline, WantLS5 string
	LiveDesired, LiveActual       string
	ShadowLS5, ShadowBase         string
	BarTime                       int64
	ATR, UpperN, LowerN, Close    float64
	PauseActive, ResumeReady      bool
}

func (s *Store) UpsertBarLog(ctx context.Context, b BarLog) error {
	pa, rr := 0, 0
	if b.PauseActive {
		pa = 1
	}
	if b.ResumeReady {
		rr = 1
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO bar_logs(symbol, bar_time, atr, upper_n, lower_n, close, want_baseline, want_ls5,
  ls5_pause_active, ls5_resume_ready, live_desired, live_actual, shadow_ls5_pos, shadow_baseline_pos)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(symbol, bar_time) DO UPDATE SET
  atr=excluded.atr, upper_n=excluded.upper_n, lower_n=excluded.lower_n, close=excluded.close,
  want_baseline=excluded.want_baseline, want_ls5=excluded.want_ls5,
  ls5_pause_active=excluded.ls5_pause_active, ls5_resume_ready=excluded.ls5_resume_ready,
  live_desired=excluded.live_desired, live_actual=excluded.live_actual,
  shadow_ls5_pos=excluded.shadow_ls5_pos, shadow_baseline_pos=excluded.shadow_baseline_pos`,
		b.Symbol, b.BarTime, b.ATR, b.UpperN, b.LowerN, b.Close, b.WantBaseline, b.WantLS5,
		pa, rr, b.LiveDesired, b.LiveActual, b.ShadowLS5, b.ShadowBase)
	return err
}

func (s *Store) PruneBarLogs(ctx context.Context, olderThan int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM bar_logs WHERE bar_time < ?`, olderThan)
	return err
}

type CurvePoint struct {
	TS                   int64   `json:"ts"`
	Reason               string  `json:"reason"`
	EquityLive           float64 `json:"equity_live"`
	EquityLiveExFunding  float64 `json:"equity_live_ex_funding"`
	UpnlLive             float64 `json:"upnl_live"`
	WalletCash           float64 `json:"wallet_cash"`
	EquityShadowLS5      float64 `json:"equity_shadow_ls5"`
	EquityShadowBaseline float64 `json:"equity_shadow_baseline"`
	GapVsLS5             float64 `json:"gap_vs_ls5"`
	CumFunding           float64 `json:"cum_funding"`
	CumFeesLive          float64 `json:"cum_fees_live"`
	CumFeesShadowLS5     float64 `json:"cum_fees_shadow_ls5"`
	MatchRateLTD         float64 `json:"match_rate_ltd"`
	SideSlipMedianBps    float64 `json:"side_slip_median_ltd_bps"`
	Verdict              string  `json:"verdict"`
}

func (s *Store) InsertCurve(ctx context.Context, p CurvePoint) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO equity_curve(ts, reason, equity_live, equity_live_ex_funding, upnl_live, wallet_cash,
  equity_shadow_ls5, equity_shadow_baseline, gap_vs_ls5, cum_funding, cum_fees_live, cum_fees_shadow_ls5,
  match_rate_ltd, side_slip_median_ltd_bps, verdict)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.TS, p.Reason, p.EquityLive, p.EquityLiveExFunding, p.UpnlLive, p.WalletCash,
		p.EquityShadowLS5, p.EquityShadowBaseline, p.GapVsLS5, p.CumFunding, p.CumFeesLive, p.CumFeesShadowLS5,
		p.MatchRateLTD, p.SideSlipMedianBps, p.Verdict)
	return err
}

func (s *Store) ListCurve(ctx context.Context, limit int) ([]CurvePoint, error) {
	if limit <= 0 {
		limit = 2000
	}
	pts, err := s.listCurveWhere(ctx, limit, `reason IN ('bar','boot')`)
	if err != nil || len(pts) > 0 {
		return pts, err
	}
	return s.listCurveWhere(ctx, limit, `1=1`)
}

func (s *Store) listCurveWhere(ctx context.Context, limit int, where string) ([]CurvePoint, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT ts, reason, equity_live, equity_live_ex_funding, upnl_live, wallet_cash,
  equity_shadow_ls5, equity_shadow_baseline, gap_vs_ls5, cum_funding, cum_fees_live, cum_fees_shadow_ls5,
  match_rate_ltd, side_slip_median_ltd_bps, verdict
FROM (
  SELECT * FROM equity_curve WHERE `+where+` ORDER BY id DESC LIMIT ?
) ORDER BY ts ASC, id ASC`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CurvePoint, 0)
	for rows.Next() {
		var p CurvePoint
		if err := rows.Scan(&p.TS, &p.Reason, &p.EquityLive, &p.EquityLiveExFunding, &p.UpnlLive, &p.WalletCash,
			&p.EquityShadowLS5, &p.EquityShadowBaseline, &p.GapVsLS5, &p.CumFunding, &p.CumFeesLive, &p.CumFeesShadowLS5,
			&p.MatchRateLTD, &p.SideSlipMedianBps, &p.Verdict); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) LatestCurve(ctx context.Context) (CurvePoint, error) {
	var p CurvePoint
	err := s.DB.QueryRowContext(ctx, `
SELECT ts, reason, equity_live, equity_live_ex_funding, upnl_live, wallet_cash,
  equity_shadow_ls5, equity_shadow_baseline, gap_vs_ls5, cum_funding, cum_fees_live, cum_fees_shadow_ls5,
  match_rate_ltd, side_slip_median_ltd_bps, verdict
FROM equity_curve ORDER BY id DESC LIMIT 1`).Scan(
		&p.TS, &p.Reason, &p.EquityLive, &p.EquityLiveExFunding, &p.UpnlLive, &p.WalletCash,
		&p.EquityShadowLS5, &p.EquityShadowBaseline, &p.GapVsLS5, &p.CumFunding, &p.CumFeesLive, &p.CumFeesShadowLS5,
		&p.MatchRateLTD, &p.SideSlipMedianBps, &p.Verdict)
	if err == sql.ErrNoRows {
		return CurvePoint{}, nil
	}
	return p, err
}

func (s *Store) TradesByProfile(ctx context.Context, profile string) ([]TradeRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+tradeCols+`
FROM trades WHERE profile=? ORDER BY id ASC`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TradeRow
	for rows.Next() {
		t, err := scanTradeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Store) TradeBySignal(ctx context.Context, symbol, profile string, signalTime int64) (*TradeRow, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+tradeCols+`
FROM trades WHERE symbol=? AND profile=? AND signal_time=?`, symbol, profile, signalTime)
	return scanTrade(row)
}

func (s *Store) SumNet(ctx context.Context, profile string) (net, fees, funding float64, err error) {
	return s.SumNetSymbol(ctx, profile, "")
}

func (s *Store) SumNetSymbol(ctx context.Context, profile, symbol string) (net, fees, funding float64, err error) {
	q := `
SELECT COALESCE(SUM(CASE WHEN outcome NOT IN ('open','rejected') THEN net ELSE 0 END),0),
       COALESCE(SUM(CASE WHEN outcome NOT IN ('open','rejected') THEN entry_fee+exit_fee ELSE 0 END),0),
       COALESCE(SUM(CASE WHEN IFNULL(outcome,'') != 'rejected' THEN funding ELSE 0 END),0)
FROM trades WHERE profile=?`
	args := []any{profile}
	if symbol != "" {
		q += ` AND symbol=?`
		args = append(args, symbol)
	}
	err = s.DB.QueryRowContext(ctx, q, args...).Scan(&net, &fees, &funding)
	return
}

func (s *Store) SumClosedFunding(ctx context.Context, profile string) (float64, error) {
	var n sql.NullFloat64
	err := s.DB.QueryRowContext(ctx, `
SELECT COALESCE(SUM(CASE WHEN outcome NOT IN ('open','rejected') THEN funding ELSE 0 END),0)
FROM trades WHERE profile=?`, profile).Scan(&n)
	return n.Float64, err
}

func (s *Store) OpenBookTrade(ctx context.Context, symbol, profile string) (*TradeRow, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+tradeCols+`
FROM trades WHERE symbol=? AND profile=? AND outcome='open' ORDER BY id DESC LIMIT 1`, symbol, profile)
	return scanTrade(row)
}

func (s *Store) CurveEquities(ctx context.Context) (live, shadow []float64, err error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT equity_live, equity_shadow_ls5 FROM equity_curve ORDER BY id ASC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l, sh float64
		if err := rows.Scan(&l, &sh); err != nil {
			return nil, nil, err
		}
		live = append(live, l)
		shadow = append(shadow, sh)
	}
	return live, shadow, rows.Err()
}

func (s *Store) HasMismatch(ctx context.Context, kind, symbol, direction string, signalTime int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `
SELECT COUNT(1) FROM mismatches WHERE kind=? AND symbol=? AND direction=? AND signal_time=?`,
		kind, symbol, direction, signalTime).Scan(&n)
	return n > 0, err
}

func (s *Store) InsertMismatch(ctx context.Context, kind, symbol, direction string, signalTime int64, note string) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO mismatches(ts, kind, symbol, direction, signal_time, note) VALUES(?,?,?,?,?,?)`,
		time.Now().Unix(), kind, symbol, direction, signalTime, note)
	return err
}

func (s *Store) ListMismatches(ctx context.Context, since int64, limit int) ([]MismatchRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT ts, kind, symbol, direction, signal_time, note FROM mismatches WHERE ts>=? ORDER BY id DESC LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MismatchRow, 0)
	for rows.Next() {
		var m MismatchRow
		if err := rows.Scan(&m.TS, &m.Kind, &m.Symbol, &m.Direction, &m.SignalTime, &m.Note); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type MismatchRow struct {
	TS         int64  `json:"ts"`
	Kind       string `json:"kind"`
	Symbol     string `json:"symbol"`
	Direction  string `json:"direction"`
	SignalTime int64  `json:"signal_time"`
	Note       string `json:"note"`
}

func (s *Store) SaveDailyReport(ctx context.Context, dateUTC, body, verdict string) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO daily_reports(date_utc, ts, body, verdict) VALUES(?,?,?,?)
ON CONFLICT(date_utc) DO UPDATE SET ts=excluded.ts, body=excluded.body, verdict=excluded.verdict`,
		dateUTC, time.Now().Unix(), body, verdict)
	return err
}

func (s *Store) LastDailyReportDate(ctx context.Context) (string, error) {
	var d string
	err := s.DB.QueryRowContext(ctx, `SELECT date_utc FROM daily_reports ORDER BY date_utc DESC LIMIT 1`).Scan(&d)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return d, err
}

func (s *Store) LatestDailyReport(ctx context.Context) (date, body, verdict string, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT date_utc, body, verdict FROM daily_reports ORDER BY date_utc DESC LIMIT 1`).
		Scan(&date, &body, &verdict)
	if err == sql.ErrNoRows {
		return "", "", "", nil
	}
	return
}
