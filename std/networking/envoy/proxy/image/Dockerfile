FROM envoyproxy/envoy:v1.22.0

WORKDIR /

COPY bootstrap-xds.yaml /bootstrap-xds.yaml

COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

CMD /entrypoint.sh