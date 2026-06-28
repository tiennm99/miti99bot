FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# SOURCE_COMMIT is baked into the binary so internal/deploynotify can DM the
# owner once per new version (parity with the Makefile build). Coolify exposes
# the commit SHA as the SOURCE_COMMIT build arg automatically — no manual
# wiring needed. For a manual build, pass it with
#   --build-arg SOURCE_COMMIT=$(git rev-parse --short HEAD)
# When unset, deploynotify treats the empty SHA as "stay silent".
ARG SOURCE_COMMIT=""
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.gitSHA=${SOURCE_COMMIT}" \
    -o /out/server \
    ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
