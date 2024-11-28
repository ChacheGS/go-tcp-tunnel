FROM golang:1.23 AS builder

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY . .

RUN make build

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/bin/go-tcp-tunnel .
USER 65532:65532

ENTRYPOINT ["./go-tcp-tunnel"]
