FROM golang:1.14 as builder

RUN mkdir /build
ADD . /build/
WORKDIR /build/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o stg-controller .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/stg-controller /stg-controller

ENTRYPOINT ["/stg-controller"]
