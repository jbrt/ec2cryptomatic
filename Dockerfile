FROM golang:1.14.3-alpine3.11 AS builder
WORKDIR /go/src/github.com/jbrt/ec2cryptomatic  
COPY . .
RUN apk add -U --no-cache ca-certificates
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ec2cryptomatic .

FROM scratch  
LABEL maintainer="julien@toshokan.fr"
WORKDIR /app/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/src/github.com/jbrt/ec2cryptomatic/ec2cryptomatic .
ENTRYPOINT ["./ec2cryptomatic"]
CMD ["--help"]  
