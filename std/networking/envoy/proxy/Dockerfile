FROM alpine:latest

WORKDIR /

COPY --from=envoyproxy/envoy:v1.22.0 /usr/local/bin/envoy /usr/local/bin/envoy

COPY bootstrap-xds.yaml /bootstrap-xds.yaml

COPY entrypoint.sh /entrypoint.sh