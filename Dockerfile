FROM --platform=$BUILDPLATFORM golang:1.24.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /emulator ./cmd/emulator


FROM alpine:3.20

WORKDIR /

RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -S app && adduser -S app -G app

RUN mkdir -p /work && chown -R app:app /work

COPY --from=builder /emulator /emulator

ENV SERVER_PORT=8086 \
    SPEC_PATH=/work/swagger.json \
    SAMPLES_DIR=/work/sample \
    LOG_LEVEL=info \
    RUNNING_ENV=docker

USER app
EXPOSE 8086
ENTRYPOINT ["/emulator"]
