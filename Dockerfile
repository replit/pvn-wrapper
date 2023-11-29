FROM golang:1.21 AS builder

WORKDIR /src/

COPY . .
ENV CGO_ENABLED=0
RUN go build -v -o /usr/local/bin/pvn-wrapper ./cmd/pvn-wrapper

FROM scratch
COPY --from=builder /usr/local/bin/pvn-wrapper /pvn-wrapper
ENTRYPOINT ["/pvn-wrapper"]
