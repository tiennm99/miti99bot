# miti99bot Integration

Future bot integration should keep winner handling explicit.

## Recommended Flow

1. Bot parses `/wheelofnamesbeta` options.
2. Bot chooses `winnerIndex` locally.
3. Bot calls `POST /api/gif`.
4. Bot sends response bytes with Telegram `sendAnimation`.
5. Bot caption uses local winner as source of truth.
6. If the service fails, bot falls back to existing local GIF renderer.

## Request

Send `Authorization: Bearer <API_TOKEN>` with the JSON body.

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

## Response

- Body: `image/gif`
- Headers:
  - `X-Wheel-Winner-Index`
  - `X-Wheel-Winner`
  - `X-Render-Duration-Ms`

`X-Wheel-Winner` is URL-encoded. Decode it before displaying if service-side
winner selection is used.
