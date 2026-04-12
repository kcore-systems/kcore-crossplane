package kcore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	insecurecreds "google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	apisv1alpha1 "github.com/kcore/kcore-crossplane/apis/kcore/v1alpha1"
	kcorepb "github.com/kcore/kcore-crossplane/gen/proto/kcore/controller/v1"
)

// Client wraps a gRPC Controller client and connection.
type Client struct {
	API  kcorepb.ControllerClient
	Conn *grpc.ClientConn
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	if c == nil || c.Conn == nil {
		return nil
	}
	return c.Conn.Close()
}

// Dial connects to the kcore controller using ProviderConfig or ClusterProviderConfig.
func Dial(ctx context.Context, kube client.Client, ref *xpv1.ProviderConfigReference, namespace string) (*Client, error) {
	if ref == nil {
		return nil, errors.New("provider config reference is nil")
	}
	var (
		endpoint string
		insecure bool
		cd       apisv1alpha1.ProviderCredentials
		pcMeta   metav1.Object
	)
	switch strings.TrimSpace(ref.Kind) {
	case "", "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, pc); err != nil {
			return nil, errors.Wrap(err, "get ProviderConfig")
		}
		endpoint = strings.TrimSpace(pc.Spec.Endpoint)
		insecure = pc.Spec.Insecure
		cd = pc.Spec.Credentials
		pcMeta = pc
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, "get ClusterProviderConfig")
		}
		endpoint = strings.TrimSpace(cpc.Spec.Endpoint)
		insecure = cpc.Spec.Insecure
		cd = cpc.Spec.Credentials
		pcMeta = cpc
	default:
		return nil, errors.Errorf("unsupported provider config kind %q", ref.Kind)
	}
	if endpoint == "" {
		return nil, errors.New("provider config endpoint is empty")
	}

	caPEM, certPEM, keyPEM, err := loadTLSMaterial(ctx, kube, pcMeta, cd)
	if err != nil {
		return nil, err
	}

	opts := []grpc.DialOption{}
	if insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecurecreds.NewCredentials()))
	} else {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		if len(caPEM) > 0 {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, errors.New("invalid ca certificate PEM")
			}
			tlsCfg.RootCAs = pool
		}
		if len(certPEM) > 0 && len(keyPEM) > 0 {
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, errors.Wrap(err, "load client certificate")
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "grpc dial")
	}
	return &Client{
		API:  kcorepb.NewControllerClient(conn),
		Conn: conn,
	}, nil
}

func loadTLSMaterial(ctx context.Context, kube client.Client, pc metav1.Object, cd apisv1alpha1.ProviderCredentials) (caPEM, certPEM, keyPEM []byte, err error) {
	switch cd.Source {
	case xpv1.CredentialsSourceNone:
		return nil, nil, nil, nil
	case xpv1.CredentialsSourceSecret:
		if cd.SecretRef == nil {
			return nil, nil, nil, errors.New("credentials secretRef is required when source is Secret")
		}
		ns := cd.SecretRef.Namespace
		if ns == "" && pc != nil {
			ns = pc.GetNamespace()
		}
		sec := &corev1.Secret{}
		if err := kube.Get(ctx, types.NamespacedName{Name: cd.SecretRef.Name, Namespace: ns}, sec); err != nil {
			return nil, nil, nil, errors.Wrap(err, "get credentials secret")
		}
		return firstKey(sec.Data, "ca.crt", "ca.pem"), firstKey(sec.Data, "tls.crt", "client.crt"), firstKey(sec.Data, "tls.key", "client.key"), nil
	default:
		return nil, nil, nil, errors.Errorf("credentials source %q is not supported (use Secret or None)", cd.Source)
	}
}

func firstKey(data map[string][]byte, keys ...string) []byte {
	for _, k := range keys {
		if v := data[k]; len(v) > 0 {
			return v
		}
	}
	return nil
}
