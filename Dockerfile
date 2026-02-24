# Stage 1: Build UI (needs full repo for graphql codegen schema refs)
FROM --platform=$BUILDPLATFORM node:20-alpine AS ui
RUN corepack enable && corepack prepare pnpm@10.18.2 --activate
WORKDIR /app
# Copy full repo — codegen.ts references ../../../pkg/coreapi/**/*.graphql
COPY ui/ ui/
COPY pkg/coreapi/ pkg/coreapi/
RUN cd ui/apps/dev-server-ui && pnpm install --frozen-lockfile && pnpm build

# Stage 2: Build Go binary with embedded UI
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
RUN apk add build-base
WORKDIR /app
COPY vendor vendor
COPY . .
# Copy built UI assets into the embed directory (matches make build-ui)
COPY --from=ui /app/ui/apps/dev-server-ui/dist/ pkg/devserver/static/
# Create placeholder for embeddocs if missing
RUN mkdir -p internal/embeddocs/website/pages/docs && \
    touch internal/embeddocs/website/pages/docs/placeholder.md
ARG TARGETARCH
ARG TARGETOS
RUN GOFLAGS=-mod=vendor GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /go/bin/inngest ./cmd/

# Stage 3: Runtime
FROM debian:stable-slim AS inngest
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata curl && \
    rm -rf /var/lib/apt/lists/* && \
    update-ca-certificates
COPY --from=build /go/bin/inngest /bin/inngest
CMD ["inngest"]
