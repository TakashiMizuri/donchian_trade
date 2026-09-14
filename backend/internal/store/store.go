package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"donchian.trade/bot/internal/strategy"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." && filepath.Dir(path) != "" {
		return nil, err
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrateTelemetry(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrateCash(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	_, err := s.DB.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS candles (
  symbol TEXT NOT NULL,
  time INTEGER NOT NULL,
  open REAL NOT NULL,
  high REAL NOT NULL,
  low REAL NOT NULL,
  close REAL NOT NULL,
  volume REAL NOT NULL DEFAULT 0,
  PRIMARY KEY (symbol, time)
);
CREATE TABLE IF NOT EXISTS trades (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  symbol TEXT NOT NULL,
  profile TEXT NOT NULL,
  direction TEXT NOT NULL,
  signal_time INTEGER NOT NULL,
  entry_time INTEGER,
  entry_price REAL,
  stop REAL,
  exit_time INTEGER,
  exit_price REAL,
  outcome TEXT,
  risk_distance REAL,
  quantity REAL,
  gross REAL,
  entry_fee REAL,
  exit_fee REAL,
  funding REAL DEFAULT 0,
  net REAL,
  consec_losses INTEGER,
  pause_until INTEGER,
  client_order_index INTEGER,
  UNIQUE(symbol, profile, signal_time)
);
CREATE TABLE IF NOT EXISTS orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  symbol TEXT NOT NULL,
  client_order_index INTEGER NOT NULL,
  exchange_order_index INTEGER,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  reduce_only INTEGER NOT NULL DEFAULT 0,
  price REAL,
  trigger_price REAL,
  quantity REAL,
  tx_hash TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS equity_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  source TEXT NOT NULL,
  equity REAL NOT NULL,
  available REAL,
  UNIQUE(ts, source)
);
CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  level TEXT NOT NULL,
  kind TEXT NOT NULL,
  message TEXT NOT NULL,
  payload TEXT
);
CREATE TABLE IF NOT EXISTS bot_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
`)
	return err
}

type SymbolState struct {
	ConsecLosses     int     `json:"consec_losses"`
	PauseUntilIdx    int     `json:"pause_until_idx"`
	PauseUntilTime   int64   `json:"pause_until_time"`
	LastBarTime      int64   `json:"last_bar_time"`
	LastSignalTime   int64   `json:"last_signal_time"`
	OpenTradeID      int64   `json:"open_trade_id"`
	SLClientOrderIdx int64   `json:"sl_client_order_idx"`
	EntryClientIdx   int64   `json:"entry_client_idx"`
	BarsSeen         int     `json:"bars_seen"`
	FundingBasis     float64 `json:"funding_basis"`
	FundingLast      float64 `json:"funding_last"`
}

type GlobalState struct {
	KillSwitch    bool    `json:"kill_switch"`
	DailyPnL      float64 `json:"daily_pnl"`
	DailyPnLDate  string  `json:"daily_pnl_date"`
	LastHeartbeat int64   `json:"last_heartbeat"`
	LastError     string  `json:"last_error"`
	WSConnected   bool    `json:"ws_connected"`
	GoLiveTS      int64   `json:"go_live_ts"`
	StartEquity   float64 `json:"start_equity"`
	LastVerdict   string  `json:"last_verdict"`
	LastReplayISO string  `json:"last_replay_iso"`
}

func (s *Store) LoadSymbolState(ctx context.Context, symbol string) (SymbolState, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM bot_state WHERE key = ?`, "sym:"+symbol).Scan(&raw)
	if err == sql.ErrNoRows {
		return SymbolState{PauseUntilIdx: -1}, nil
	}
	if err != nil {
		return SymbolState{}, err
	}
	var st SymbolState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return SymbolState{}, err
	}
	return st, nil
}

func (s *Store) SaveSymbolState(ctx context.Context, symbol string, st SymbolState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
INSERT INTO bot_state(key, value, updated_at) VALUES(?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		"sym:"+symbol, string(b), time.Now().Unix())
	return err
}

func (s *Store) LoadGlobal(ctx context.Context) (GlobalState, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM bot_state WHERE key = ?`, "global").Scan(&raw)
	if err == sql.ErrNoRows {
		return GlobalState{}, nil
	}
	if err != nil {
		return GlobalState{}, err
	}
	var st GlobalState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return GlobalState{}, err
	}
	return st, nil
}

func (s *Store) SaveGlobal(ctx context.Context, st GlobalState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
INSERT INTO bot_state(key, value, updated_at) VALUES(?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		"global", string(b), time.Now().Unix())
	return err
}

func (s *Store) UpsertCandles(ctx context.Context, symbol string, bars []strategy.Bar) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO candles(symbol, time, open, high, low, close, volume)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(symbol, time) DO UPDATE SET
  open=excluded.open, high=excluded.high, low=excluded.low,
  close=excluded.close, volume=excluded.volume`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, b := range bars {
		if _, err := stmt.ExecContext(ctx, symbol, b.Time, b.Open, b.High, b.Low, b.Close, b.Volume); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadCandles(ctx context.Context, symbol string) ([]strategy.Bar, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT time, open, high, low, close, volume FROM candles WHERE symbol = ? ORDER BY time ASC`, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []strategy.Bar
	for rows.Next() {
		var b strategy.Bar
		if err := rows.Scan(&b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) LastCandleTime(ctx context.Context, symbol string) (int64, error) {
	var t sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT MAX(time) FROM candles WHERE symbol = ?`, symbol).Scan(&t)
	if err != nil {
		return 0, err
	}
	if !t.Valid {
		return 0, nil
	}
	return t.Int64, nil
}

type TradeRow struct {
	ID               int64
	Symbol           string
	Profile          string
	Direction        string
	SignalTime       int64
	EntryTime        sql.NullInt64
	EntryPrice       sql.NullFloat64
	Stop             sql.NullFloat64
	ExitTime         sql.NullInt64
	ExitPrice        sql.NullFloat64
	Outcome          sql.NullString
	RiskDistance     sql.NullFloat64
	Quantity         sql.NullFloat64
	Gross            sql.NullFloat64
	EntryFee         sql.NullFloat64
	ExitFee          sql.NullFloat64
	Funding          sql.NullFloat64
	Net              sql.NullFloat64
	ConsecLosses     sql.NullInt64
	PauseUntil       sql.NullInt64
	ClientOrderIndex sql.NullInt64
	EntryPxShadow    float64
	EntryPxLive      float64
	ExitPxShadow     float64
	ExitPxLive       float64
}

const tradeCols = `id, symbol, profile, direction, signal_time, entry_time, entry_price, stop, exit_time, exit_price,
  outcome, risk_distance, quantity, gross, entry_fee, exit_fee, funding, net, consec_losses, pause_until, client_order_index,
  COALESCE(entry_px_shadow,0), COALESCE(entry_px_live,0), COALESCE(exit_px_shadow,0), COALESCE(exit_px_live,0)`

func (s *Store) TryOpenTrade(ctx context.Context, symbol, profile, direction string, signalTime int64, entryTime int64, entry, stop, risk, qty float64, clientIdx int64) (id int64, inserted bool, err error) {
	res, err := s.DB.ExecContext(ctx, `
INSERT OR IGNORE INTO trades(
  symbol, profile, direction, signal_time, entry_time, entry_price, stop,
  outcome, risk_distance, quantity, client_order_index, entry_px_shadow)
VALUES(?,?,?,?,?,?,?,'open',?,?,?,?)`,
		symbol, profile, direction, signalTime, entryTime, entry, stop, risk, qty, clientIdx, entry)
	if err != nil {
		return 0, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, false, nil
	}
	id, err = res.LastInsertId()
	return id, true, err
}

func (s *Store) CloseTrade(ctx context.Context, id int64, exitTime int64, exitPrice float64, outcome string, gross, entryFee, exitFee, funding, net float64, consec int, pauseUntil int64) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE trades SET exit_time=?, exit_price=?, outcome=?, gross=?, entry_fee=?, exit_fee=?,
  funding=?, net=?, consec_losses=?, pause_until=?,
  exit_px_live=CASE WHEN ?>0 THEN ? ELSE exit_px_live END
WHERE id=?`,
		exitTime, exitPrice, outcome, gross, entryFee, exitFee, funding, net, consec, pauseUntil, exitPrice, exitPrice, id)
	return err
}

func (s *Store) SetEntryLive(ctx context.Context, id int64, shadowPx, livePx, entryFee float64) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE trades SET entry_px_shadow=?, entry_px_live=?, entry_fee=? WHERE id=?`,
		shadowPx, livePx, entryFee, id)
	return err
}

func (s *Store) SetExitShadow(ctx context.Context, id int64, shadowPx float64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE trades SET exit_px_shadow=? WHERE id=?`, shadowPx, id)
	return err
}

func (s *Store) SetTradeFunding(ctx context.Context, id int64, funding float64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE trades SET funding=? WHERE id=?`, funding, id)
	return err
}

func (s *Store) OpenLiveTrade(ctx context.Context, symbol string) (*TradeRow, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+tradeCols+`
FROM trades WHERE symbol=? AND profile='live' AND outcome='open' ORDER BY id DESC LIMIT 1`, symbol)
	return scanTrade(row)
}

func (s *Store) ListTrades(ctx context.Context, limit int) ([]TradeRow, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+tradeCols+`
FROM trades ORDER BY id DESC LIMIT ?`, limit)
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

func (s *Store) ClosedLiveTrades(ctx context.Context) ([]TradeRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+tradeCols+`
FROM trades WHERE profile='live' AND outcome IS NOT NULL AND outcome != 'open' ORDER BY id ASC`)
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

type scanner interface {
	Scan(dest ...any) error
}

func scanTrade(row scanner) (*TradeRow, error) {
	var t TradeRow
	err := row.Scan(&t.ID, &t.Symbol, &t.Profile, &t.Direction, &t.SignalTime, &t.EntryTime, &t.EntryPrice, &t.Stop,
		&t.ExitTime, &t.ExitPrice, &t.Outcome, &t.RiskDistance, &t.Quantity, &t.Gross, &t.EntryFee, &t.ExitFee,
		&t.Funding, &t.Net, &t.ConsecLosses, &t.PauseUntil, &t.ClientOrderIndex,
		&t.EntryPxShadow, &t.EntryPxLive, &t.ExitPxShadow, &t.ExitPxLive)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanTradeRows(rows *sql.Rows) (*TradeRow, error) {
	return scanTrade(rows)
}

func (s *Store) InsertOrder(ctx context.Context, symbol string, clientIdx int64, kind, status string, reduceOnly bool, price, trigger, qty float64, txHash string) error {
	now := time.Now().Unix()
	ro := 0
	if reduceOnly {
		ro = 1
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO orders(symbol, client_order_index, kind, status, reduce_only, price, trigger_price, quantity, tx_hash, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`, symbol, clientIdx, kind, status, ro, price, trigger, qty, txHash, now, now)
	return err
}

func (s *Store) UpdateOrderStatus(ctx context.Context, clientIdx int64, status string, exchangeIdx int64) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE orders SET status=?, exchange_order_index=?, updated_at=? WHERE client_order_index=?`,
		status, exchangeIdx, time.Now().Unix(), clientIdx)
	return err
}

func (s *Store) InsertEquity(ctx context.Context, source string, equity, available float64) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT OR REPLACE INTO equity_snapshots(ts, source, equity, available) VALUES(?,?,?,?)`,
		time.Now().Unix(), source, equity, available)
	return err
}

func (s *Store) LatestEquity(ctx context.Context, source string) (EquityPoint, error) {
	var p EquityPoint
	var avail sql.NullFloat64
	err := s.DB.QueryRowContext(ctx, `
SELECT ts, equity, available FROM equity_snapshots WHERE source=? ORDER BY ts DESC LIMIT 1`, source).
		Scan(&p.TS, &p.Equity, &avail)
	if err == sql.ErrNoRows {
		return EquityPoint{}, nil
	}
	if err != nil {
		return EquityPoint{}, err
	}
	p.Available = avail.Float64
	return p, nil
}

func (s *Store) ListEquity(ctx context.Context, source string, limit int) ([]EquityPoint, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT ts, equity, available FROM (
  SELECT ts, equity, available FROM equity_snapshots WHERE source=? ORDER BY ts DESC LIMIT ?
) AS recent ORDER BY ts ASC`, source, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EquityPoint
	for rows.Next() {
		var p EquityPoint
		var avail sql.NullFloat64
		if err := rows.Scan(&p.TS, &p.Equity, &avail); err != nil {
			return nil, err
		}
		p.Available = avail.Float64
		out = append(out, p)
	}
	return out, rows.Err()
}

type EquityPoint struct {
	TS        int64   `json:"ts"`
	Equity    float64 `json:"equity"`
	Available float64 `json:"available"`
}

func (s *Store) InsertEvent(ctx context.Context, level, kind, message string, payload any) error {
	var raw string
	if payload != nil {
		b, err := json.Marshal(payload)
		if err == nil {
			raw = string(b)
		}
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO events(ts, level, kind, message, payload) VALUES(?,?,?,?,?)`,
		time.Now().Unix(), level, kind, message, raw)
	return err
}

func (s *Store) ListEvents(ctx context.Context, limit int) ([]EventRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT ts, level, kind, message FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		if err := rows.Scan(&e.TS, &e.Level, &e.Kind, &e.Message); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type EventRow struct {
	TS      int64  `json:"ts"`
	Level   string `json:"level"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (s *Store) NextClientOrderIndex(ctx context.Context) (int64, error) {
	var raw sql.NullString
	if err := s.DB.QueryRowContext(ctx, `SELECT value FROM bot_state WHERE key='client_order_seq'`).Scan(&raw); err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	next := time.Now().UnixMilli() % (1 << 40)
	if raw.Valid {
		var prev int64
		if _, err := fmt.Sscanf(raw.String, "%d", &prev); err == nil && prev >= next {
			next = prev + 1
		}
	}
	if next <= 0 {
		next = 1
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO bot_state(key, value, updated_at) VALUES('client_order_seq', ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		fmt.Sprintf("%d", next), time.Now().Unix())
	return next, err
}
