FROM golang:1.24 AS builder

ADD . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /bin/dhcpeeper main.go

FROM alpine:3.20.3
COPY --from=builder /bin/dhcpeeper /bin/dhcpeeper
RUN apk add --no-cache dhcrelay tzdata
EXPOSE 67 67/udp
ENTRYPOINT ["dhcrelay", "-d"]