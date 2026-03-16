# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/go-serve .

FROM alpine
COPY --from=builder /out/go-serve /go-serve

EXPOSE 9443
USER 65534
ENTRYPOINT ["/go-serve"]
