FROM golang:1.26.5-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/followingfeed .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/followingfeed /app/followingfeed

ENV GIN_MODE=release
EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/followingfeed"]
