package private

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	instance "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/private/instance/instancev1betagrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
)

type InstanceServiceClient struct {
	instance.InstanceServiceClient
}

func MakeInstanceClient() (*InstanceServiceClient, error) {
	md, err := metadata.InstanceMetadataFromFile()
	if err != nil {
		return nil, err
	}

	tlsConfig, err := makeTLSConfigFromInstance(md)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.DialContext(context.Background(), "https://"+md.InstanceEndpoint, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}

	cli := instance.NewInstanceServiceClient(conn)
	return &InstanceServiceClient{cli}, nil
}

func makeTLSConfigFromInstance(md metadata.InstanceMetadata) (*tls.Config, error) {
	caCert, err := os.ReadFile(md.Certs.HostPublicPemPath)
	if err != nil {
		return nil, fnerrors.New("could not ca open certificate file: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	publicCert, err := os.ReadFile(md.Certs.PublicPemPath)
	if err != nil {
		return nil, fnerrors.New("could not public cert file: %v", err)
	}

	privateKey, err := os.ReadFile(md.Certs.PrivateKeyPath)
	if err != nil {
		return nil, fnerrors.New("could not private key file: %v", err)
	}

	keyPair, err := tls.X509KeyPair(publicCert, privateKey)
	if err != nil {
		return nil, fnerrors.New("could not load instance keys: %v", err)
	}

	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{keyPair},
	}, nil
}
