# syntax=docker/dockerfile:1
# One multi-stage file for every backend unit.
#   docker build -f docker/Dockerfile --build-arg SERVICE=read-api --build-arg TARGET=memory .
# The builder cross-compiles on the build platform, so no emulation is needed
# for linux/amd64 and linux/arm64. Only go.mod, go.sum, api/, cmd/, and
# internal/ are copied, so private files can never reach an image.

FROM --platform=$BUILDPLATFORM golang:1.27@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS build
ARG SERVICE
ARG TARGET
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN test -n "$SERVICE" && test -n "$TARGET"
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -tags "$TARGET" -trimpath -ldflags="-s -w" -o /out/service ./cmd/$SERVICE

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
ARG SERVICE
ARG TARGET
ARG COMMIT=unknown
LABEL org.opencontainers.image.source="https://github.com/easterthebunny/pubcommittee" \
      org.opencontainers.image.revision="$COMMIT" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.title="$SERVICE" \
      org.opencontainers.image.description="PubCommittee $SERVICE ($TARGET target)"
COPY --from=build /out/service /service
USER nonroot:nonroot
ENTRYPOINT ["/service"]
