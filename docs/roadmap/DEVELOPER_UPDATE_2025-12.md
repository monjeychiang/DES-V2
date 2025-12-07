# DES v2.0 – Developer Update (2025‑12)

This note captures recent backend/frontend changes so you don’t have to reverse‑engineer them from the code.

---

## 1. Auth & Session Model

- **New endpoints**
  - `POST /api/auth/register` – create user, returns `{ token, user }`.
  - `POST /api/auth/login` – login with email/password, returns `{ token, user }`.
- **JWT**
  - All `/api/*` routes (except `/api/system/status` and `/api/auth/*`) now require:
    - Header: `Authorization: Bearer <JWT>`.
  - Claims: `uid` (user ID), expiry ~72h, signing key `JWT_SECRET` from env.
- **Failure codes (JSON)**
  - `MISSING_TOKEN`, `INVALID_AUTH_HEADER`, `INVALID_TOKEN`, `MISSING_CREDENTIALS`, `INVALID_EMAIL`,
    `EMAIL_ALREADY_REGISTERED`, `INVALID_CREDENTIALS`, `INTERNAL_ERROR`.
- **Frontend**
  - Token stored in `localStorage` as `des_token`.
  - Axios interceptor automatically attaches `Authorization: Bearer ...`.

---

## 2. Per‑User Exchange Connections

- **Schema**
  - Table `users`:
    - `id`, `email` (unique), `password_hash`, `created_at`, `updated_at`.
  - Table `connections`:
    - `id`, `user_id`, `exchange_type`, `name`, `api_key`, `api_secret`, `is_active`, timestamps.
  - Table `strategy_instances` (new columns):
    - `user_id` (owner), `connection_id` (bound connection), `status` (ACTIVE/PAUSED/STOPPED).
- **Endpoints**
  - `GET /api/connections` – list current user’s connections.
  - `POST /api/connections` – create connection:
    - Body: `{ name, exchange_type, api_key, api_secret }`.
  - `DELETE /api/connections/:id` – soft‑delete (`is_active=0`) for current user.
  - `PUT /api/strategies/:id/binding` – bind strategy to user + connection:
    - Body: `{ connection_id }` (may be empty string to unbind).
    - Enforces:
      - Strategy must belong to current user or be unowned.
      - Connection must belong to current user and be `is_active=1`.
- **Frontend**
  - “Exchange Connections” panel in Dashboard:
    - Add new connection (name, type, API Key/Secret).
    - List & deactivate existing connections.
  - Strategy list:
    - “Connection” column – dropdown to select a connection per strategy.
    - Value saved via `PUT /api/strategies/:id/binding`.

---

## 3. Per‑Strategy Gateway Routing (Live vs Dry‑Run)

- **Live mode (`DRY_RUN=false`)**
  - Orders now route per strategy:
    - For each `Order` with `StrategyInstanceID`:
      1. Lookup `strategy_instances.connection_id`.
      2. Resolve corresponding row in `connections` (must be active).
      3. Build (or reuse cached) Binance gateway based on `exchange_type`:
         - `binance-spot` → spot client.
         - `binance-usdtfut` → USDT futures client.
         - `binance-coinfut` → COIN futures client.
      4. Submit order via that gateway.
  - **No more fallback to global `.env` key for bound strategies**:
    - If no valid gateway can be resolved for a strategy:
      - Order is stored in DB.
      - Status set to `REJECTED`.
      - `EventOrderRejected` is emitted with reason `"no gateway for order"`.
  - Global gateway from `.env` is still used for:
    - Orders not associated with a strategy (very rare / internal paths).
- **Dry‑Run mode (`DRY_RUN=true`)**
  - Uses `DryRunExecutor`:
    - Persists orders and emits events.
    - Does **not** hit any external gateway (`SkipExchange` flag).
    - Simulates PnL and balance in memory.
  - Strategy may be unbound in Dry‑Run; binding is only required for real trading.

---

## 4. System Status & Strategy Views

- **System status API**
  - `GET /api/system/status` (no auth required):
    - `mode`: `DRY_RUN` / `LIVE`.
    - `dry_run`: boolean.
    - `venue`: current global venue (`binance-spot` / futures / `none`).
    - `symbols`: configured symbol list.
    - `use_mock_feed`: bool.
    - `version`: from `APP_VERSION` env or `v2.0-dev`.
    - `server_time`: current UTC time.
- **Frontend**
  - `SystemStatusBar` component on Dashboard:
    - DRY_RUN / LIVE badge.
    - Venue, symbols, feed type, version, server time.
  - Strategy list:
    - Shows `status` badge (ACTIVE/PAUSED/STOPPED).
    - Start/Pause/Stop/Panic actions.
    - Start button:
      - In DRY_RUN: always allowed.
      - In LIVE: disabled when `connection_id` is empty; tooltip explains why.

---

## 5. Strategy Performance API & UI

- **API**
  - `GET /api/strategies/:id/performance?from=YYYY-MM-DD&to=YYYY-MM-DD`
  - Behavior:
    - Default range: last 30 days if no dates passed.
    - Based on `trades` + `orders` tables.
    - Per‑day PnL = Σ(SELL notional − BUY notional − fee).
    - Cumulative equity computed as running sum of daily PnL.
    - Response:
      ```json
      {
        "strategy_id": "ma_btc_1",
        "from": "2025-11-01",
        "to": "2025-12-01",
        "daily": [
          { "date": "2025-11-01", "PNL": 120.5, "Equity": 120.5 },
          { "date": "2025-11-02", "PNL": -30.0, "Equity": 90.5 }
        ],
        "total_pnl": 90.5
      }
      ```
    - Access control uses the same checks as start/stop: only owner (or unowned) strategies are visible.
- **Frontend**
  - New `PerformanceModal`:
    - Triggered from Strategy list via “View” link.
    - Shows:
      - Daily PnL as a horizontal bar list (green/red).
      - Cumulative equity as a per‑day value list.
  - This is intentionally minimal and uses the existing data model—no chart library dependency.

---

## 6. Environment Variables (delta)

See `docs/setup/ENV_VARIABLES_GUIDE.md` for the full list; key ones relevant to recent changes:

- **Execution / DB**
  - `DRY_RUN` – `true` for simulation only, `false` for live trading.
  - `DRY_RUN_INITIAL_BALANCE` – starting simulated balance in Dry‑Run.
  - `DRY_RUN_DB_PATH` – optional alternate DB path for Dry‑Run.
  - `DB_PATH` – primary SQLite DB path for live/normal mode.
- **Auth / security**
  - `JWT_SECRET` – HMAC key for signing JWT access tokens.
  - `LICENSE_SERVER` – reserved for license server integration (if used).
- **Binance**
  - `BINANCE_TESTNET` – whether to use Binance testnet.
  - `BINANCE_API_KEY`, `BINANCE_API_SECRET` – **global** key; still required for some system‑level flows, but not used for per‑strategy trading once connections are bound.
  - `ENABLE_BINANCE_TRADING`, `ENABLE_BINANCE_USDT_FUTURES`, `ENABLE_BINANCE_COIN_FUTURES` – feature toggles for each venue.

---

## 7. Where to Look in the Code

- Auth & users: `backend/cmd/trading-core/internal/api/auth.go`, `pkg/db/models.go` (User).
- Connections & binding: `internal/api/controllers.go` (`listConnections`, `createConnection`, `deactivateConnection`, `updateStrategyBinding`), `pkg/db/models.go` (Connection).
- Strategy ownership & access checks: `internal/api/controllers.go:canAccessStrategy`.
- Per‑strategy routing: `internal/order/executor.go` (`gatewayForOrder`, `gatewayForStrategy`).
- Dry‑Run executor: `internal/order/dry_run.go`.
- Performance API: `internal/api/controllers.go:getStrategyPerformance`.
- Frontend:
  - Auth/Login: `frontend/src/components/Login.jsx`, `frontend/src/App.jsx`, `frontend/src/api.js`.
  - Dashboard & status: `frontend/src/components/Dashboard.jsx`, `SystemStatusBar.jsx`.
  - Connections: `frontend/src/components/ConnectionsPanel.jsx`.
  - Strategies: `frontend/src/components/StrategyList.jsx`, `EditStrategyModal.jsx`, `PerformanceModal.jsx`.

This file is meant as a delta‑log; for full architecture context, combine it with:
- `docs/process/DEVELOPER_ONBOARDING.md`
- `docs/architecture/SYSTEM_ARCHITECTURE.md`
- `docs/roadmap/DEVELOPMENT_ROADMAP_DES_V2.md`
