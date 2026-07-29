FROM golang:1.26.5-alpine AS builder
WORKDIR /src

# The monkeyd module renders PDFs with an embedded TrueType font, and the
# runtime image below ships no fonts at all. DejaVuSans covers the Latin
# Extended Additional block that Vietnamese diacritics live in; it is installed
# here and copied into the final stage.
RUN apk add --no-cache font-dejavu

# The monkeyd-crawler submodule is resolved through a `replace` directive, so
# its go.mod must be present before `go mod download` can read the build list.
# Only the module files are copied here, keeping this layer cached across
# ordinary source edits.
COPY go.mod go.sum ./
COPY third_party/monkeyd-crawler/go.mod third_party/monkeyd-crawler/go.sum ./third_party/monkeyd-crawler/
RUN go mod download

COPY . .

# The deploy-notify commit SHA comes from Coolify's SOURCE_COMMIT runtime env,
# not from a build arg. Keep it out of the build so Docker cache survives across
# commits. The binary is built plain.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/server \
    ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/server /server
# pdfout.FindFont() probes system font paths; this is one of the paths it knows.
COPY --from=builder /usr/share/fonts/dejavu/DejaVuSans.ttf /usr/share/fonts/dejavu/DejaVuSans.ttf
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
