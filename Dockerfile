FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod ./
COPY *.go ./
COPY public ./public
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /find-filament .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /find-filament /find-filament
USER 65532:65532
ENV PORT=3000 HOST=0.0.0.0
EXPOSE 3000
ENTRYPOINT ["/find-filament"]
