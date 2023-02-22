FROM golang:1.20 AS builder

WORKDIR /src/

COPY . .
RUN go build -v -o /usr/local/bin/pvn-wrapper ./cmd/pvn-wrapper

FROM debian:bullseye-slim
COPY --from=builder /usr/local/bin/pvn-wrapper /usr/local/bin/pvn-wrapper

ENTRYPOINT ["pvn-wrapper"]
