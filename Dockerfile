FROM golang:1.24.3 AS builder

COPY . /gccs
WORKDIR /gccs

RUN make package

FROM alpine:3
WORKDIR /root/
COPY --from=builder /gccs/gccs .

CMD ["./gccs"]
EXPOSE 80
