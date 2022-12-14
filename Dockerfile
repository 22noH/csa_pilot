FROM golang:1.20rc1 AS builder
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY . .
RUN go mod download
RUN go build -o main 

WORKDIR /dist
RUN cp /build/main .

# FROM scratch
# COPY --from=builder /dist/main .
# ENTRYPOINT ["/main"]