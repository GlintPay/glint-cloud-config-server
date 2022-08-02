FROM golang:1.18.4 as builder

RUN mkdir -p /services

COPY . /services/glint-cloud-config-server
WORKDIR /services/glint-cloud-config-server

RUN make package

FROM alpine:3
WORKDIR /root/
COPY --from=builder /services/glint-cloud-config-server/gccs .

CMD ["./gccs"]
EXPOSE 80
