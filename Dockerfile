# Build natively on the host and cross-compile (CGO is off), so other target platforms need no emulation
FROM --platform=$BUILDPLATFORM golang:1.27.2 AS builder
ARG TARGETOS
ARG TARGETARCH

COPY . /gccs
WORKDIR /gccs

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} make package

FROM alpine:3
WORKDIR /root/
COPY --from=builder /gccs/gccs .

CMD ["./gccs"]
EXPOSE 80
