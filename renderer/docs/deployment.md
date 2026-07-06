# Deployment

## Recommendation

Use a self-hosted/container runtime for v1. Static-only hosts are not enough
because `/api/gif` must render GIF bytes on the server with Remotion.

Good first targets:

- Coolify Docker app
- Fly.io
- Railway
- Render
- VPS with Docker
- Google Cloud Run generic container

Avoid for v1:

- Cloudflare Workers/Pages
- pure static Vercel/Netlify deploys

## Runtime

Required env vars:

```sh
PORT=3000
HOST=0.0.0.0
MAX_CONCURRENT_RENDERS=1
RENDER_TIMEOUT_MS=15000
MAX_OPTIONS=32
MAX_OPTION_CHARS=40
API_TOKEN=change-me
```

`compose.yml` forwards these values from the shell with `${VAR:-default}`
fallbacks, so unset or empty shell variables use the defaults above.

Start with 1-2 vCPU and 1-2 GB RAM. Increase only after render benchmarks show
the service is CPU-bound or concurrency-limited.

Local non-Docker render smoke requires Chrome Headless Shell shared libraries,
including `libnspr4` and `libnss3`. Prefer Docker for consistent verification.
The Docker image runs `pnpm browser:ensure` during build so production requests
do not need to download Chrome Headless Shell on first render.

`RENDER_TIMEOUT_MS` is a total render timeout. Values below `7000` are raised to
`7000` because Remotion's browser timeout has that minimum.

Set `API_TOKEN` for every production deployment. The server refuses to start
with `NODE_ENV=production` unless the token is configured.

## Health

```sh
curl http://localhost:3000/api/healthz
```

## Render Test

```sh
curl -X POST http://localhost:3000/api/gif \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer change-me' \
  --output wheel.gif \
  --data '{"options":["alice","bob","carol"],"winnerIndex":1}'
```
