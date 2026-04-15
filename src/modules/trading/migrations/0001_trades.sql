CREATE TABLE trading_trades (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  symbol TEXT NOT NULL,
  side TEXT NOT NULL CHECK (side IN ('buy','sell')),
  qty INTEGER NOT NULL,
  price_vnd INTEGER NOT NULL,
  ts INTEGER NOT NULL
);
CREATE INDEX idx_trading_trades_user_ts ON trading_trades(user_id, ts DESC);
CREATE INDEX idx_trading_trades_ts ON trading_trades(ts);
