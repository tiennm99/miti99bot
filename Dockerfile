FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# GIT_SHA is baked into the binary so internal/deploynotify can DM the owner
# once per new version (parity with the Makefile build). Coolify exposes the
# commit SHA as a build arg — pass it with
#   --build-arg GIT_SHA=$(git rev-parse --short HEAD)
# When unset, deploynotify treats the empty SHA as "stay silent".
ARG GIT_SHA=""
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.gitSHA=${GIT_SHA}" \
    -o /out/server \
    ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
