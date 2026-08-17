# wheelofnames

Self-hosted API that renders wheel-of-names GIF animations with Remotion.

## API

```http
POST /api/gif
Content-Type: application/json
Accept: image/gif
Authorization: Bearer change-me
```

```json
{
  "options": ["alice", "bob", "carol"],
  "winnerIndex": 1,
  "durationMs": 6500,
  "holdMs": 1200,
  "fps": 15,
  "size": 512,
  "theme": "classic"
}
```

Response is `image/gif` with winner metadata headers:

- `X-Wheel-Winner-Index`
- `X-Wheel-Winner`
- `X-Render-Duration-Ms`

`X-Wheel-Winner` is URL-encoded so non-ASCII labels are safe in HTTP headers.

## Local

Install dependencies and Chromium once:

```sh
npm install
npm run browser:ensure
```

Start the local API:

```sh
npm run dev
```

### Generate GIF files locally

Generate the quick smoke fixture at the git-ignored path
`fixtures/smoke.gif`:

```sh
npm run render:smoke
```

Generate the complete fixture set at `fixtures/smoke.gif`,
`fixtures/vietnamese.gif`, and `fixtures/sixteen-options.gif`:

```sh
npm run render:fixtures
```

Render a custom GIF directly without starting the API server:

```powershell
npm run render:local -- `
  --output wheel.gif `
  --option "Chiều nay uống CraneTea" `
  --option "Chiều nay uống CraneTea" `
  --option "Cà phê" `
  --winner 1
```

macOS, Linux, or Git Bash:

```sh
npm run render:local -- \
  --output wheel.gif \
  --option "Chiều nay uống CraneTea" \
  --option "Chiều nay uống CraneTea" \
  --option "Cà phê" \
  --winner 1
```

`--winner` is a zero-based index and is random when omitted. Run
`npm run render:local -- --help` for duration, hold, FPS, size, theme, and timeout
options. The documented root `wheel.gif` and `fixtures/*.gif` outputs are
git-ignored and safe to delete; custom output paths may need their own ignore
rule.

### Verify

Run the API smoke test and quality gates:

```sh
npm run api:smoke
npm run lint
npm run typecheck
npm test
```

## Deploy

Use a container runtime first. Static-only platforms cannot satisfy
`POST /api/gif` because Remotion server rendering needs Node, Chromium/runtime
dependencies, and FFmpeg/compositor support.

```sh
docker build -t wheelofnames .
docker run --rm -p 3000:3000 -e API_TOKEN=change-me wheelofnames
```

Recommended starting resources: 1-2 vCPU and 1-2 GB RAM, with
`MAX_CONCURRENT_RENDERS=1`. `RENDER_TIMEOUT_MS` defaults to `15000` and is
raised to Remotion's `7000ms` browser timeout floor when configured lower.
`API_TOKEN` is required when `NODE_ENV=production`.
