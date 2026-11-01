FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY . .
ARG SOURCE_REPOSITORY
RUN set -eu; \
    if ! git rev-parse --verify HEAD >/dev/null 2>&1 && [ -n "$SOURCE_REPOSITORY" ]; then \
        git clone --depth 1 --no-checkout "$SOURCE_REPOSITORY" /tmp/revision; \
        cp -a /tmp/revision/.git /src/.git; \
        git reset --mixed HEAD; \
        git diff --exit-code HEAD; \
    fi; \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.buildCommit=$(git rev-parse HEAD 2>/dev/null || echo dev)" -o /find-filament .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /find-filament /find-filament
USER 65532:65532
ENV PORT=3000 HOST=0.0.0.0
EXPOSE 3000
ENTRYPOINT ["/find-filament"]
