FROM golang:1.26.4-trixie AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o find-ten ./cmd/server

FROM alpine:latest

WORKDIR /app

# Copy the built binary from the builder stage to the workdir of the final image
COPY --from=builder /app/find-ten . 

# Copy the static files to the workdir of the final image
COPY --from=builder /app/static ./static

EXPOSE 8080

CMD [ "./find-ten" ]


