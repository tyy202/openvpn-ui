# Build the checked-out source, including the bilingual UI. No release tarball required.
FROM golang:1.26.5-alpine3.23 AS builder
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .
RUN go test -mod=vendor ./internal/deployconfig ./internal/accesspolicy \
 && CGO_ENABLED=1 go build -mod=vendor -trimpath -o /out/openvpn-ui .

FROM alpine:3.23
RUN apk add --no-cache bash ca-certificates curl jq easy-rsa openvpn openssl \
    iptables iproute2 oath-toolkit-oathtool libqrencode-tools \
 && chmod 755 /usr/share/easy-rsa/*
WORKDIR /opt/openvpn-ui
COPY --from=builder /out/openvpn-ui ./openvpn-ui
COPY views ./views
COPY static ./static
COPY swagger ./swagger
COPY conf ./conf
COPY build/assets/app.conf ./conf/app.conf
COPY build/assets/ /opt/scripts/
COPY deploy/ /opt/deploy/
# Windows checkouts may contain CRLF. Normalize scripts in the image.
RUN sed -i 's/\r$//' /opt/scripts/*.sh /opt/deploy/*.sh /opt/openvpn-ui/conf/*.tpl \
 && chmod +x /opt/scripts/*.sh /opt/deploy/*.sh \
 && cp /opt/deploy/qrencode.sh /opt/scripts/qrencode \
 && mkdir -p db \
 && ln -s /etc/openvpn/pki /usr/share/easy-rsa/pki
EXPOSE 8080/tcp 1194/udp
ENTRYPOINT ["/opt/deploy/entrypoint.sh"]
CMD ["ui"]
