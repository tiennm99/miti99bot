# wheelofnames

Self-hosted API that renders wheel-of-names GIF animations and gacha wish
MP4 animations with Remotion.

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

### Gacha wish

```http
POST /api/gacha
Content-Type: application/json
Accept: video/mp4
Authorization: Bearer change-me
```

```json
{
  "label": "Pizza",
  "rarity": 5,
  "fps": 24,
  "width": 640
}
```

Renders a 7-second wish animation in the style of a gacha game: a meteor
coloured by rarity (blue 3★, purple 4★, gold 5★) flies in from the left across
a night sky and bursts in a white flash where the rank emblem appears, and the label is revealed beside a rank emblem (`B`,
`A`, `S`) with its stars popping in. Each tier is louder than the one below:
4★ adds a bigger meteor, a lens flare, impact shake, and a double shockwave;
5★ adds a rainbow sunburst before landing, a gold sky flood, a starburst, counter
rotating rays, falling sparkles, and a sheen across the emblem. The per-tier
table lives in `src/remotion/gacha-timeline.js`.
`rarity` is required; `fps` is `24` or `30`; `width` is `640` (360 tall) or
`854` (480 tall). Optional `seed` (integer, `0` to `2147483647`) lays out the
twinkling stars and particles; when omitted the service picks a random one, so
every roll draws a different sky. The caller chooses the result and its rarity — the service
only draws it.

### Gacha wish beta

`POST /api/gachabeta` takes the same body and returns the same response as
`/api/gacha`, rendering a 10.5-second beta style instead, cut like an anime
gacha game's six-star wish:

1. The camera dives through a ring of painterly clouds into a swirling eye,
   then a beam rushes past.
2. A meteor slows to a stop over a sea of clouds inside a rainbow halo,
   sheds sparkles, and bursts into a starburst under a red flash.
3. Crystal comets fall through the daytime sky while a red star joins them
   and strikes in a white flash.
4. A black silhouette of the rank emblem sheds shards on a red disc, then the
   splash card shows the emblem, a name plate with the request's `label` and
   rank, and one star per rarity level popping in.

The caller randomly chooses the food and sends it as `label` (for example,
`"Bún bò"`). As on `/api/gacha`, `rarity` picks the rank (`B` for 3★, `A` for
4★, `S` for 5★), the star count, and the tint of the meteor head and emblem
(blue, purple, gold).

Response is a silent H.264 `video/mp4` (Telegram plays it as an animation)
with `X-Gacha-Rarity` and `X-Render-Duration-Ms` headers. Both routes share the
`MAX_CONCURRENT_RENDERS` slots. All visuals are drawn procedurally; no game
assets are used.

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

Generate the quick smoke fixtures at the git-ignored paths
`fixtures/smoke.gif` and `fixtures/gacha-5-star.mp4`:

```sh
npm run render:smoke
```

Generate the complete fixture set at `fixtures/smoke.gif`,
`fixtures/vietnamese.gif`, `fixtures/sixteen-options.gif`, and
`fixtures/gacha-{3,4,5}-star.mp4`:

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
options. The documented root `wheel.gif` and `fixtures/*.gif`/`*.mp4` outputs are
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
