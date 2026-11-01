FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.buildCommit=$(git rev-parse HEAD 2>/dev/null || echo dev)" -o /find-filament .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /find-filament /find-filament
USER 65532:65532
ENV PORT=3000 HOST=0.0.0.0
EXPOSE 3000
ENTRYPOINT ["/find-filament"]
