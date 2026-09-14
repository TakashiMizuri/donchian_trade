package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func (s *Store) migrateCash() error {
	_, err := s.DB.Exec(`
CREATE TABLE IF NOT EXISTS cash_flows (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  amount REAL NOT NULL,
  kind TEXT NOT NULL,
  live_equity REAL,
  live_wallet REAL,
  explained REAL,
  note TEXT
);
CREATE INDEX IF NOT EXISTS idx_cash_flows_ts ON cash_flows(ts);`)
	return err
}

type CashFlow struct {
	ID         int64   `json:"id"`
	TS         int64   `json:"ts"`
	Amount     float64 `json:"amount"`
	Kind       string  `json:"kind"`
	LiveEquity float64 `json:"live_equity"`
	LiveWallet float64 `json:"live_wallet"`
	Explained  float64 `json:"explained"`
	Note       string  `json:"note"`
}

type CashWatch struct {
	Inited       bool    `json:"inited"`
	LastWallet   float64 `json:"last_wallet"`
	LastRealized float64 `json:"last_realized"`
	LastOpenFees float64 `json:"last_open_fees"`
}

func (s *Store) LoadCashWatch(ctx context.Context) (CashWatch, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM bot_state WHERE key=?`, "cash_watch").Scan(&raw)
	if err == sql.ErrNoRows {
		return CashWatch{}, nil
	}
	if err != nil {
		return CashWatch{}, err
	}
	var w CashWatch
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return CashWatch{}, err
	}
	return w, nil
}

func (s *Store) SaveCashWatch(ctx context.Context, w CashWatch) error {
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
INSERT INTO bot_state(key, value, updated_at) VALUES('cash_watch', ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		string(b), time.Now().Unix())
	return err
}

func (s *Store) InsertCashFlow(ctx context.Context, f CashFlow) (int64, error) {
	if f.TS == 0 {
		f.TS = time.Now().Unix()
	}
	res, err := s.DB.ExecContext(ctx, `
INSERT INTO cash_flows(ts, amount, kind, live_equity, live_wallet, explained, note)
VALUES(?,?,?,?,?,?,?)`, f.TS, f.Amount, f.Kind, f.LiveEquity, f.LiveWallet, f.Explained, f.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) SumCash(ctx context.Context) (float64, error) {
	var n sql.NullFloat64
	err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount),0) FROM cash_flows`).Scan(&n)
	return n.Float64, err
}

func (s *Store) ListCashFlows(ctx context.Context, limit int) ([]CashFlow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, ts, amount, kind, live_equity, live_wallet, explained, note
FROM cash_flows ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CashFlow
	for rows.Next() {
		var f CashFlow
		var note sql.NullString
		if err := rows.Scan(&f.ID, &f.TS, &f.Amount, &f.Kind, &f.LiveEquity, &f.LiveWallet, &f.Explained, &note); err != nil {
			return nil, err
		}
		f.Note = note.String
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) LatestCashFlow(ctx context.Context) (CashFlow, error) {
	var f CashFlow
	var note sql.NullString
	err := s.DB.QueryRowContext(ctx, `
SELECT id, ts, amount, kind, live_equity, live_wallet, explained, note
FROM cash_flows ORDER BY id DESC LIMIT 1`).
		Scan(&f.ID, &f.TS, &f.Amount, &f.Kind, &f.LiveEquity, &f.LiveWallet, &f.Explained, &note)
	if err == sql.ErrNoRows {
		return CashFlow{}, nil
	}
	if err != nil {
		return CashFlow{}, err
	}
	f.Note = note.String
	return f, nil
}

func (s *Store) CashFlowCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM cash_flows`).Scan(&n)
	return n, err
}
