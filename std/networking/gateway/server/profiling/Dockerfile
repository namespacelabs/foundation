ARG ENVOY_VERSION
FROM envoyproxy/envoy-debug:$ENVOY_VERSION AS envoy

RUN apt-get update && apt-get install -y curl npm g++ git libelf-dev libcap-dev

RUN npm install -g @bazel/bazelisk

RUN git clone https://github.com/google/perf_data_converter.git /usr/src/perf_data_converter

WORKDIR /usr/src/perf_data_converter

RUN bazel build src:perf_to_profile

RUN cp bazel-bin/src/perf_to_profile /usr/bin/.

WORKDIR /

RUN apt-get update && apt-get install -y graphviz

RUN curl -OL https://golang.org/dl/go1.18.4.linux-amd64.tar.gz

RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.4.linux-amd64.tar.gz

ENV GOBIN /usr/local/go/bin

ENV PATH $PATH:${GOBIN}

RUN go install github.com/google/pprof@latest

ENTRYPOINT ["pprof", "-http=0.0.0.0:8888", "/usr/local/bin/envoy"]