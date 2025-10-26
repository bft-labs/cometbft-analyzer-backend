# cometbft-analyzer-backend

Backend service for the CometBFT Analyzer. It manages users, projects, and simulations, accepts CometBFT log uploads, invokes the ETL pipeline to normalize logs into MongoDB, and serves metrics and events to the frontend.

Part of the CometBFT Analyzer suite:

- cometbft-log-etl: Parses CometBFT logs and writes per-simulation DBs
- cometbft-analyzer-backend: This service (HTTP API + orchestration)
- cometbft-analyzer-frontend: Web UI that consumes this API
- cometbft-analyzer-types: Shared Go types and statistics used by ETL/backend

Note: This project is under active development. APIs and schemas may evolve.

## Quickstart

- Requirements:
  - Go 1.21+
  - MongoDB (local or remote)
  - cometbft-log-etl binary on PATH as `cometbft-log-etl`

1) Run MongoDB (defaults to `mongodb://localhost:27017`).

2) Build and install the ETL binary (from the cometbft-log-etl repo):

```bash
git clone https://github.com/bft-labs/cometbft-log-etl
cd cometbft-log-etl
go build -o cometbft-log-etl .
mv cometbft-log-etl /usr/local/bin/  # or add to your PATH
```

3) Start the backend:

```bash
go run .
# or
PORT=8080 MONGODB_URI=mongodb://localhost:27017 go run .
```

The server listens on `:8080` by default and exposes routes under `/v1`.

## Configuration

- `MONGODB_URI`: MongoDB connection string (default: `mongodb://localhost:27017`).
- `PORT`: HTTP listen port (default: `8080`).
- `.env`: Optionally load these from a local `.env` file.

### CORS and Security

The service enables:
- Security headers (X-Frame-Options, X-Content-Type-Options, etc.)
- Basic request validation for content types and Accept header
- CORS allowlist: `http://localhost:3000`, `http://localhost:3001`, `https://yourdomain.com`
- Rate limiting: 6000 req/min with burst 10 (IP-based)

Adjust `middleware/` code to customize these policies for your deployment.

## Data Model and Flow

- Control plane DB: `consensus_visualizer` stores metadata collections:
  - `users`, `projects`, `simulations`
- Per-simulation DB: named by the simulation’s Mongo ObjectID (hex). The ETL writes collections such as:
  - `events` and/or `consensus_events` (normalized events for metrics)
  - `vote_latencies` (derived vote message latencies)
  - `network_latency_nodepair_summary`, `network_latency_node_stats` (network latency rollups)

File storage (local filesystem):
- Uploaded logs are stored under `uploads/user_<userId>/project_<projectId>/simulation_<simId>/`
- A `processed/` subfolder is created post-ETL for future outputs

Processing pipeline:
1) Create a simulation (optionally upload logs in the same request)
2) Upload log files (multipart)
3) Trigger processing (`POST /v1/simulations/:id/process`) which runs `cometbft-log-etl -dir <sim_dir> -simulation <sim_id>`
4) Query metrics and events from the per-simulation database

## API Overview

Base URL: `/v1`
Content types: `application/json` for JSON; `multipart/form-data` for file uploads.
Time window query params: unless noted, metrics accept `from` and `to` as RFC3339 timestamps; if omitted, defaults to last 1 minute.

### Users
- `POST /users` – Create user: `{ username, email }`
- `GET /users` – List users
- `GET /users/:userId` – Get user
- `DELETE /users/:userId` – Delete user

### Projects
- `POST /users/:userId/projects` – Create project: `{ name, description }`
- `GET /users/:userId/projects` – List projects for a user
- `GET /projects/:projectId` – Get project
- `PUT /projects/:projectId` – Update project: `{ name?, description? }`
- `DELETE /projects/:projectId` – Delete project

### Simulations
- `POST /users/:userId/projects/:projectId/simulations`
  - JSON: `{ name, description }`
  - or multipart: fields `name`, `description`, files `logfiles[]`
  - If files are provided, processing status is set and ETL may be kicked off automatically.
- `GET /users/:userId/simulations` – List simulations for a user
- `GET /projects/:projectId/simulations` – List simulations for a project
- `GET /simulations/:id` – Get simulation (includes status and processing result)
- `PUT /simulations/:id` – Update simulation: `{ name?, description? }`
- `DELETE /simulations/:id` – Delete simulation (removes uploaded files, leaves DBs intact)
- `POST /simulations/:id/upload` – Upload additional log files (multipart `logfiles[]`)
- `POST /simulations/:id/process` – Trigger ETL on uploaded logs (async)

### Events and Metrics (per simulation)
All routes below are prefixed with `/simulations/:id` and query the per-simulation DB.

- `GET /events`
  - Cursor pagination over normalized consensus events.
  - Query: `from`, `to` (RFC3339), `limit` (default 10000, max 50000), `cursor` (next), `before` (prev), `segment` (1-indexed), `includeTotalCount=true`.
  - Returns `{ data: Event[], pagination: { ... } }`.

- `GET /metrics/latency/votes`
  - Paginated vote latencies above a threshold percentile within time window.
  - Query: `from`, `to`, `page` (default 1), `perPage` (default 100, max 1000), `threshold` (`p50|p95|p99`, default `p95`).

- `GET /metrics/latency/pairwise`
  - Sender→receiver latency percentiles (p50, p95, p99) within time window.

- `GET /metrics/latency/timeseries`
  - Per-block time series of vote propagation latency (ms). Uses send/receive pairs.

- `GET /metrics/latency/stats`
  - Latency histogram (bucketAuto) and jitter (stddev) per sender→receiver pair.

- `GET /metrics/messages/success_rate`
  - Send vs receive counts and delivery ratio per height and pair.

- `GET /metrics/latency/end_to_end`
  - End-to-end consensus latency per block height (p50/p95) from EnteringNewRound to ReceivedCompleteProposalBlock.

- `GET /metrics/vote/statistics`
  - Aggregated vote statistics by sender/receiver/type including p50/p90/p95/p99 and spike percentage.

- `GET /metrics/network/latency/stats`
  - Node-pair network latency stats (precomputed by ETL). Returns array of NodePairLatencyStats.

- `GET /metrics/network/latency/node-stats`
  - Network latency node statistics (per-node rollups).

- `GET /metrics/network/latency/overview`
  - Overall weighted p95, highest-contributing message type/node, plus per-type and per-node contributions.

## Example Workflow (cURL)

```bash
# 1) Create a user
curl -sX POST localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.org"}'

# 2) Create a project for the user
curl -sX POST localhost:8080/v1/users/<userId>/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","description":"cometbft test"}'

# 3) Create a simulation and upload logs in one go
curl -sX POST localhost:8080/v1/users/<userId>/projects/<projectId>/simulations \
  -H 'Content-Type: multipart/form-data' \
  -F 'name=demo-sim' \
  -F 'description=normal run' \
  -F 'logfiles=@/path/to/node1.log' \
  -F 'logfiles=@/path/to/node2.log'

# 4) (Optional) Trigger processing explicitly
curl -sX POST localhost:8080/v1/simulations/<simulationId>/process

# 5) Query metrics
curl -s 'localhost:8080/v1/simulations/<simulationId>/metrics/latency/pairwise?from=2024-01-01T00:00:00Z&to=2024-01-01T00:10:00Z'
```

## Project Structure

- `main.go` – Server setup, routes, middleware, Mongo connection
- `handlers/` – HTTP handlers (users, projects, simulations, metrics, events)
- `metrics/` – Query pipelines over per-simulation collections
- `types/` – Response and domain types (imports cometbft-analyzer-types)
- `middleware/` – Security, CORS, rate limit, request validation
- `db/` – Mongo connection helper
- `utils/` – File layout helpers and time window parsing
- `uploads/` – Local storage for uploaded logs (gitignored)

## Notes and Tips

- Ensure `cometbft-log-etl` is available on PATH for processing. The backend calls it with:
  `cometbft-log-etl -dir <simulation_dir> -simulation <simulation_id>`
- Frontend consumers should honor rate limits and use `from`/`to` windows for heavy queries.
- The events API supports cursor pagination (`cursor` and `before`) and segment offsets for large timelines.
- If you adjust CORS origins or rate limits, update code in `middleware/`.

## Contributing

- Keep changes scoped and avoid coupling API handlers to specific storage schemas beyond what metrics require.
- If you extend per-simulation collections or metrics, add endpoints and document them here.
- PRs improving safety, performance, and observability are welcome.

## License

Licensed under the Apache License, Version 2.0. See the LICENSE file for details.
