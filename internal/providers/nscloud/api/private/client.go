package private

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"time"

	instance "buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/private/instance/instancev1betaconnect"
	"connectrpc.com/connect"
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

	client := &http.Client{
		Timeout: time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	cli := instance.NewInstanceServiceClient(
		client,
		"https://"+md.InstanceEndpoint,
		connect.WithGRPC(), //TODO check if okay or connect protocol?
	)

	return &InstanceServiceClient{cli}, nil
}

func makeTLSConfigFromInstance(md metadata.InstanceMetadata) (*tls.Config, error) {
	caCert, err := os.ReadFile(md.Certs.HostPublicPemPath)
	if err != nil {
		return nil, fnerrors.New("could not ca open certificate file: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	keyPair, err := tls.LoadX509KeyPair(md.Certs.PublicPemPath, md.Certs.PrivateKeyPath)
	if err != nil {
		return nil, fnerrors.New("could not load instance keys: %v", err)
	}

	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{keyPair},
	}, nil
}
