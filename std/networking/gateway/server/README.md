# Envoy Networking Gateway

We run [Envoy](https://www.envoyproxy.io/) as a HTTP-gRPC gateway with updates to Envoy
orchestrated by a Kubernetes controller wrapping Envoy's [go control plane](https://github.com/envoyproxy/go-control-plane).

## Pre-built Envoy Docker images

We leverage the following images:

- Default [envoyproxy/envoy](https://hub.docker.com/r/envoyproxy/envoy/tags/): Release binary with
symbols stripped on top of an Ubuntu Bionic base.

- [envoyproxy/envoy-debug](https://hub.docker.com/r/envoyproxy/envoy-debug/tags/) for profiling
and debugging: Release binary with debug symbols on top of an Ubuntu Bionic base.

 See [pre-built docker images](https://www.envoyproxy.io/docs/envoy/latest/start/install#pre-built-envoy-docker-images) for additional images.

## Debugging the controller

[--controller_debug](https://github.com/namespacelabs/foundation/blob/30863ba3e03271b7e17cb4bf905795ce178a5e68/std/networking/gateway/controller/main.go#L35) enables xDS gRPC server debug logging, giving visibility into each Envoy snapshot update. All Kubernetes events and reconciler actions are also logged in debug mode.

![debug_controller](https://user-images.githubusercontent.com/102962107/178251463-d54d994c-f5d7-45e1-a4d3-f8c28a757f32.png)

## Debugging Envoy

Dynamically change the debug level for all loggers:

```bash
 curl -X POST localhost:19000/logging?level=debug
```

Dynamically change the log level for specific loggers:

```bash
 curl -X POST localhost:19000/logging?paths=filter:debug,http2:debug,http:debug
```

Visit localhost:19000/logging to see all active loggers.

```
  admin: info
  alternate_protocols_cache: info
  aws: info
  assert: info
  backtrace: info
  cache_filter: info
  client: info
  config: info
  connection: info
  conn_handler: info
  decompression: info:
```


[--envoy_debug](https://github.com/namespacelabs/foundation/blob/86564d64985fb536543bbb8e8660455e899960da/std/networking/gateway/server/configure/main.go#L30) sets the envoy log level to `debug` and additionally enables the fine-grain logger with file level log control and runtime update at administration interface.

![debug_envoy](https://user-images.githubusercontent.com/102962107/179016003-039a66d5-93d4-40b5-ac34-7a6c6c0f450b.gif)

## Profiling Envoy

Please note that you need to update [container.securitycontext.yaml](https://github.com/namespacelabs/foundation/blob/main/runtime/kubernetes/defaults/container.securitycontext.yaml#L1) in privileged
mode and rebuild `nsdev` before profiling an Envoy started with `nsdev dev internal/testdata/server/gogrpc`
to allow overriding sysctl defaults for gauging CPU traces.

```yaml
privileged: true
allowPrivilegeEscalation: true
readOnlyRootFilesystem: false
```

Exec into a shell in the gateway:

```bash
nsdev t kubectl exec -- -it gateway-sun4qtee50l61888bdj0-8648b4f64d-nthv7 -c gateway -- bash
```

We need to download and `make` perf from src since we get a `WARNING: perf not found for kernel ...`
if we try installing `linux-tools-generic` in the Envoy container.

```bash
apt-get update && apt-get install -y build-essential git flex bison

git clone --depth 1 https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git

cd linux/tools/perf

make

cp perf /usr/bin

echo -1 > /proc/sys/kernel/perf_event_paranoid

perf record -F 49 -g -o /tmp/envoy.perf -p 1 -- sleep 60

perf report
```

Copy the generated profile out of the container:

```bash
 nsdev t kubectl cp -- gateway-sun4qtee50l61888bdj0-8648b4f64d-nthv7:/tmp/envoy.perf /tmp/envoy.perf -c gateway
```

## Reading the perf file with pprof

We launch `pprof` with the `perf` data collected from a production Envoy process in a docker container built off the [envoyproxy/envoy-debug](https://hub.docker.com/r/envoyproxy/envoy-debug/tags/) base with the version passed along as a docker build argument.

```bash
docker build -t envoy-perf-pprof --build-arg ENVOY_VERSION=v1.22.0 profiling/

docker run -p 8888:8888 -v /tmp/envoy.perf:/root/envoy.perf envoy-perf-pprof /root/envoy.perf
```

Navigate to port `8888` to view and drill down into the profile:

![pprof_envoy](https://user-images.githubusercontent.com/102962107/179289106-76b1976a-486c-4b06-822c-e711de46383f.png)
