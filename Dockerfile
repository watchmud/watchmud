# WatchMUD: a static binary and its content, on an image with nothing else in
# it -- so an image version pins the code and the rules and zones together.
# Built and pushed to ghcr.io by .github/workflows/build.yaml; `make
# docker-build` builds one locally.

FROM golang:1.27 AS build
WORKDIR /src

# modules first, so a code change doesn't re-download them
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# what the binary reports itself as: the release tag, or branch-and-commit,
# passed by .github/workflows/build.yaml. "dev" for a build by hand.
ARG VERSION=dev
# CGO off: a static binary, which is what lets the runtime image be distroless.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/watchmud ./cmd/watchmud
# the smoke test deploy.sh runs after a restart; same image, so it walks the
# world this version shipped with
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/watchmud-bot ./cmd/watchmud-bot
# the inhabitants: the bots service in deploy/compose.yaml runs this
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/watchmud-bots ./cmd/watchmud-bots

# distroless/static: no shell, no package manager, a non-root user. Nothing to
# exec into -- `docker compose logs` is how you see what it's doing.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/watchmud /app/watchmud
COPY --from=build /out/watchmud-bot /app/watchmud-bot
COPY --from=build /out/watchmud-bots /app/watchmud-bots
COPY content /app/content
COPY deploy/app.yaml /app/app.yaml

EXPOSE 4000
USER nonroot:nonroot
# exec form, so watchmud is PID 1 and gets docker stop's SIGTERM directly --
# that is what flushes the write-behind saves before it exits.
ENTRYPOINT ["/app/watchmud", "-config", "/app/app.yaml"]
