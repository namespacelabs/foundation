module namespacelabs.dev/foundation

go 1.18

require (
	cuelang.org/go v0.4.2
	filippo.io/age v1.0.0
	github.com/andybalholm/brotli v1.0.4
	github.com/aws/aws-sdk-go-v2 v1.15.0
	github.com/aws/aws-sdk-go-v2/config v1.13.1
	github.com/aws/aws-sdk-go-v2/service/ecr v1.14.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.14.0
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/cespare/xxhash/v2 v2.1.2
	github.com/containerd/containerd v1.6.1
	github.com/docker/cli v20.10.12+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-units v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fsnotify/fsnotify v1.5.1
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/google/go-containerregistry v0.7.1-0.20211203164431-c75901cce627
	github.com/gorilla/csrf v1.7.1
	github.com/gorilla/handlers v0.0.0-20150720190736-60c7bfde3e33
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.3
	github.com/iancoleman/strcase v0.2.0
	github.com/jackc/pgx/v4 v4.15.0
	github.com/jhump/protoreflect v1.10.3
	github.com/karrick/godirwalk v1.16.1
	github.com/kr/text v0.2.0
	github.com/mattn/go-zglob v0.0.3
	github.com/miekg/dns v1.1.46
	github.com/moby/buildkit v0.10.0
	github.com/morikuni/aec v1.0.0
	github.com/opencontainers/image-spec v1.0.2
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.32.1
	github.com/protocolbuffers/txtpbfmt v0.0.0-20201118171849-f6a6b3f636fc
	github.com/rs/zerolog v1.26.1
	github.com/soheilhy/cmux v0.1.5
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.1
	github.com/tailscale/depaware v0.0.0-20210622194025-720c4b409502
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.29.0
	go.opentelemetry.io/otel v1.4.1
	go.opentelemetry.io/otel/exporters/jaeger v1.4.1
	go.opentelemetry.io/otel/sdk v1.4.1
	go.opentelemetry.io/otel/trace v1.4.1
	golang.org/x/exp v0.0.0-20220328175248-053ad81199eb
	golang.org/x/mod v0.6.0-dev.0.20211013180041-c96bc1413d57
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220310020820-b874c991c1a5
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/tools v0.1.9
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/api v0.63.0
	google.golang.org/genproto v0.0.0-20220118154757-00ab72f36ad5
	google.golang.org/grpc v1.44.0
	google.golang.org/protobuf v1.27.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
	namespacelabs.dev/go-ids v0.0.0-20220216144417-79021816502e
	sigs.k8s.io/yaml v1.3.0
	tailscale.com v1.22.2
)

require (
	cloud.google.com/go v0.99.0 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.9.0 // indirect
	github.com/aws/smithy-go v1.11.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cncf/udpa/go v0.0.0-20210930031921-04548b0d99d4 // indirect
	github.com/cncf/xds/go v0.0.0-20211130200136-a8f946100490 // indirect
	github.com/cockroachdb/apd/v2 v2.0.1 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/continuity v0.2.2 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.11.2 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.0+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/envoyproxy/go-control-plane v0.10.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.6.2 // indirect
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.11.0 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.2.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.10.0 // indirect
	github.com/jackc/puddle v1.2.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jxskiss/base62 v1.1.0 // indirect
	github.com/klauspost/compress v1.15.0 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/signal v0.6.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mpvl/unique v0.0.0-20150818121801-cbe035fff7de // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pkg/diff v0.0.0-20210226163009-20ebb0f2a09e // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20220115021204-b19f7f9cb274 // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea // indirect
	github.com/tonistiigi/vt100 v0.0.0-20210615222946-8066bb97264f // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.4.1 // indirect
	go.opentelemetry.io/proto/otlp v0.12.0 // indirect
	golang.org/x/crypto v0.0.0-20211215165025-cf75a172585e // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/klog/v2 v2.30.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	k8s.io/utils v0.0.0-20211203121628-587287796c64 // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

replace github.com/docker/docker => github.com/docker/docker v20.10.3-0.20220121014307-40bb9831756f+incompatible
