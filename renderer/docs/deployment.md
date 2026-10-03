# Deployment

## miti99bot compose

The root `compose.yml` builds this folder as the `renderer` service and points
the bot at it over the compose network. The API has no authentication: it is
reachable only from inside that network, so never publish its port or attach a
domain to it. The rest of this page covers running the service on its own.

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

Env vars, all optional (defaults shown):

```sh
RENDERER_PORT=3000
RENDERER_HOST=0.0.0.0
RENDERER_MAX_CONCURRENT_RENDERS=1
RENDERER_RENDER_TIMEOUT_MS=15000
RENDERER_MAX_OPTIONS=32
RENDERER_MAX_OPTION_CHARS=40
```

The root `compose.yml` forwards the tuning values with `${VAR:-default}`
fallbacks, so unset or empty variables use the defaults above.

Start with 1-2 vCPU and 1-2 GB RAM. Increase only after render benchmarks show
the service is CPU-bound or concurrency-limited.

Local non-Docker render smoke requires Chrome Headless Shell shared libraries,
including `libnspr4` and `libnss3`. Prefer Docker for consistent verification.
The Docker image runs `npm run browser:ensure` during build so production requests
do not need to download Chrome Headless Shell on first render.

`RENDERER_RENDER_TIMEOUT_MS` is a total render timeout. Values below `7000` are raised to
`7000` because Remotion's browser timeout has that minimum.

## Health

```sh
curl http://localhost:3000/api/healthz
```

## Render Test

```sh
curl -X POST http://localhost:3000/api/gif \
  -H 'content-type: application/json' \
  --output wheel.gif \
  --data '{"options":["alice","bob","carol"],"winnerIndex":1}'
```
