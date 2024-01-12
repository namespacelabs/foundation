package private

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	instance "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/private/instance/instancev1betagrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
)

type InstanceServiceClient struct {
	instance.InstanceServiceClient
}

func MakeInstanceClient(ctx context.Context) (*InstanceServiceClient, error) {
	md, err := metadata.FetchInstanceMetadata()
	if err != nil {
		return nil, err
	}

	// TODO remove
	console.DebugWithTimestamp(ctx, "loaded metadata: %+v\n", md)

	tlsConfig, err := makeTLSConfigFromInstance(ctx, md)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.DialContext(ctx, md.InstanceEndpoint, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}

	cli := instance.NewInstanceServiceClient(conn)
	return &InstanceServiceClient{cli}, nil
}

func makeTLSConfigFromInstance(ctx context.Context, md metadata.InstanceMetadata) (*tls.Config, error) {
	caCert, err := os.ReadFile(md.Certs.HostPublicPemPath)
	if err != nil {
		return nil, fnerrors.New("could not ca open certificate file: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fnerrors.New("failed to append ca certificate to pool: %v", err)
	}

	publicCert, err := os.ReadFile(md.Certs.PublicPemPath)
	if err != nil {
		return nil, fnerrors.New("could not public cert file: %v", err)
	}

	privateKey, err := os.ReadFile(md.Certs.PrivateKeyPath)
	if err != nil {
		return nil, fnerrors.New("could not private key file: %v", err)
	}

	cert, err := tls.X509KeyPair(publicCert, privateKey)
	if err != nil {
		return nil, fnerrors.New("could not load instance keys: %v", err)
	}

	// TODO remove
	console.DebugWithTimestamp(ctx, "ca cert: %v\n", string(caCert))
	console.DebugWithTimestamp(ctx, "public cert: %v\n", string(publicCert))
	console.DebugWithTimestamp(ctx, "private key: %v\n", string(privateKey))

	return &tls.Config{
		RootCAs: caCertPool,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		},
	}, nil
}
