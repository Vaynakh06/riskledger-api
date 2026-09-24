# RiskLedger

RiskLedger is the core backend service for portfolio accounting, trade tracking, position calculations, and risk-limit monitoring.

## Goals

- manage portfolios and trades;
- calculate positions and PnL;
- evaluate risk limits;
- expose a clean REST API;
- serve as the main Go project in the portfolio.

## Stack

- Go
- PostgreSQL
- short-lived bearer tokens with HttpOnly refresh cookies
- Docker Compose
- REST API

## Production run

1. Copy `.env.example` to `.env` and replace every placeholder with a secret value.
2. Start the stack:

```bash
docker compose --env-file .env up --build -d
```

3. Verify readiness:

```bash
curl http://localhost:8080/health
```

The container entrypoint applies all idempotent migrations on every startup, including existing Render databases. Never commit `.env`, production credentials, or database dumps.

The API refuses to start outside `DEMO_MODE` unless both `DATABASE_URL` and a 32-character minimum `JWT_SECRET` are present. `DEMO_MODE=true` is for local demos only and uses volatile memory storage.

## Local test run

```bash
go test ./...
go vet ./...
```

## Local demo on Windows

For a quick API walkthrough without Docker or PostgreSQL, open PowerShell in the repository directory and run:

```powershell
$env:DEMO_MODE="true"
$env:PORT="8080"
go run ./cmd/api
```

Keep that terminal running. In a second PowerShell window, check the service:

```powershell
Invoke-RestMethod http://localhost:8080/health
```

The demo database contains `demo@riskledger.dev` with password `demo-password`. Request a token and call a protected endpoint:

```powershell
$login = Invoke-RestMethod -Method Post `
	-Uri http://localhost:8080/api/v1/auth/login `
	-ContentType "application/json" `
	-Body '{"email":"demo@riskledger.dev","password":"demo-password"}'

$headers = @{ Authorization = "Bearer $($login.token)" }
Invoke-RestMethod http://localhost:8080/api/v1/summary -Headers $headers
Invoke-RestMethod http://localhost:8080/api/v1/portfolios -Headers $headers
Invoke-RestMethod http://localhost:8080/api/v1/risk-limits -Headers $headers
```

To create your own account in demo mode:

```powershell
Invoke-RestMethod -Method Post `
	-Uri http://localhost:8080/api/v1/auth/register `
	-ContentType "application/json" `
	-Body '{"email":"you@example.com","password":"strong-password"}'
```

Stop the server with `Ctrl+C`. Demo data is in memory and disappears after restart.

## Web dashboard

With the API running, open [http://localhost:8080/](http://localhost:8080/) in a browser. The site includes:

- account registration and sign in;
- portfolio creation and switching;
- portfolio value and PnL summary;
- live positions and trade history;
- trade entry form;
- risk-limit creation and evaluation.

The frontend is served by the same Go process from `web/`, so there is no separate frontend server for local use.

## Deploy to Render

This repository is ready for a Docker-based Render Web Service. Use the included `render.yaml` as the deployment template, or create a new Render Web Service with these env vars:

- `PORT=8080`
- `DEMO_MODE=false`
- `JWT_SECRET=<32+ random chars>`
- `DATABASE_URL=<Render Postgres connection string>`
- `MARKET_DATA_URL=https://api.coingecko.com/api/v3/simple/price`
- optional `MARKET_DATA_API_KEY` for CoinGecko rate limits;
- optional SMTP, Telegram, Slack, and webhook variables from `.env.example` for real alert delivery.

Render will build the Docker image from `Dockerfile`, then the app will apply SQL migrations from `migrations/` before starting the server. After deployment, open the generated Render URL in a browser.

## API

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET|POST /api/v1/portfolios`
- `POST /api/v1/trades`
- `GET /api/v1/positions`
- `GET /api/v1/analytics`
- `GET /api/v1/summary`
- `GET|POST /api/v1/risk-limits`
- `GET /api/v1/risk-evaluate`
- `POST /api/v1/risk-scenario`
- `GET|POST /api/v1/risk-alert-rules`
- `POST /api/v1/risk-alerts/evaluate`
- `GET /api/v1/risk-alert-events`
- `GET /api/v1/audit-log`
- `GET|POST /api/v1/cash`
- `GET /api/v1/performance-report`
- `GET|POST /api/v1/benchmarks`
- `GET|POST /api/v1/allocations`
- `POST /api/v1/fx-rates`
- `POST /api/v1/broker-sync`
- `POST /api/v1/auth/2fa/enable`

Protected endpoints require `Authorization: Bearer <token>`. Portfolio endpoints are owner-scoped; the API also sets security headers, throttles repeated login failures, caps auth body sizes, and rotates refresh sessions in an HttpOnly cookie.

Performance reports now include cash, total return, TWR, IRR, benchmark metadata, and allocation targets. Broker sync uses the configured `BROKER_API_URL` contract and remains disabled when credentials are absent. TOTP enrollment is available through the authenticated 2FA endpoint; login accepts the `code` field when 2FA is enabled.

Trade journal fields include strategy, comma-separated tags, notes, and checklist completion. Analytics summarizes the selected portfolio by strategy, tags, trade count, and checklist discipline.

## Scaling notes

The service is stateless when running with PostgreSQL, so API replicas can be placed behind a load balancer. Before a public launch, run load tests, configure TLS at the ingress, add centralized logs/metrics, backups, secret management, and a migration job for existing database volumes.
