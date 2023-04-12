FROM registry.k8s.io/ingress-nginx/opentelemetry:v20230312-helm-chart-4.5.2-28-g66a760794@sha256:40f766ac4a9832f36f217bb0e98d44c8d38faeccbfe861fbc1a76af7e9ab257f AS otel

FROM registry.k8s.io/ingress-nginx/controller:v1.7.0@sha256:7612338342a1e7b8090bef78f2a04fffcadd548ccaabe8a47bf7758ff549a5f7

COPY --from=otel /etc/nginx/modules/otel_ngx_module.so /modules_mount/etc/nginx/modules/otel/otel_ngx_module.so