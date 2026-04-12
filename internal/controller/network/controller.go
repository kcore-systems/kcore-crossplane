package network

import (
	"context"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apisv1alpha1 "github.com/kcore/kcore-crossplane/apis/kcore/v1alpha1"
	kcorepb "github.com/kcore/kcore-crossplane/gen/proto/kcore/controller/v1"
	"github.com/kcore/kcore-crossplane/internal/controller/kcore"
)

// SetupGated registers the Network controller.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Network controller"))
		}
	}, apisv1alpha1.NetworkGroupVersionKind)
	return nil
}

// Setup adds the reconciler.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(apisv1alpha1.NetworkGroupKind)
	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*apisv1alpha1.Network](&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}
	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}
	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}
	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		rec := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &apisv1alpha1.NetworkList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(rec); err != nil {
			return errors.Wrap(err, "register MR state metrics")
		}
	}
	r := managed.NewReconciler(mgr, resource.ManagedKind(apisv1alpha1.NetworkGroupVersionKind), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&apisv1alpha1.Network{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *apisv1alpha1.Network) (managed.TypedExternalClient[*apisv1alpha1.Network], error) {
	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}
	cli, err := kcore.Dial(ctx, c.kube, cr.Spec.ProviderConfigReference, cr.GetNamespace())
	if err != nil {
		return nil, err
	}
	return &external{api: cli.API, closer: cli}, nil
}

type external struct {
	api    kcorepb.ControllerClient
	closer *kcore.Client
}

func (e *external) Disconnect(_ context.Context) error { return e.closer.Close() }

func netName(cr *apisv1alpha1.Network) string {
	if en := meta.GetExternalName(cr); en != "" {
		return en
	}
	return strings.TrimSpace(cr.Spec.ForProvider.Name)
}

func (e *external) Observe(ctx context.Context, cr *apisv1alpha1.Network) (managed.ExternalObservation, error) {
	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	name := netName(cr)
	if name == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	resp, err := e.api.ListNetworks(ctx, &kcorepb.ListNetworksRequest{
		TargetNode: strings.TrimSpace(cr.Spec.ForProvider.TargetNode),
	})
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	for _, n := range resp.GetNetworks() {
		if n.GetName() == name {
			cr.Status.AtProvider.NodeID = n.GetNodeId()
			cr.Status.SetConditions(xpv1.Available())
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: networkUpToDate(n, cr)}, nil
		}
	}
	return managed.ExternalObservation{ResourceExists: false}, nil
}

func networkUpToDate(n *kcorepb.NetworkInfo, cr *apisv1alpha1.Network) bool {
	p := cr.Spec.ForProvider
	if n.GetExternalIp() != p.ExternalIP || n.GetGatewayIp() != p.GatewayIP {
		return false
	}
	if n.GetInternalNetmask() != p.InternalNetmask {
		return false
	}
	if strings.TrimSpace(n.GetNetworkType()) != strings.TrimSpace(p.NetworkType) && p.NetworkType != "" {
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, cr *apisv1alpha1.Network) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	p := cr.Spec.ForProvider
	if strings.TrimSpace(p.Name) == "" {
		return managed.ExternalCreation{}, errors.New("spec.forProvider.name is required")
	}
	req := &kcorepb.CreateNetworkRequest{
		Name:              p.Name,
		ExternalIp:        p.ExternalIP,
		GatewayIp:         p.GatewayIP,
		InternalNetmask:   p.InternalNetmask,
		TargetNode:        p.TargetNode,
		AllowedTcpPorts:   p.AllowedTcpPorts,
		AllowedUdpPorts:   p.AllowedUdpPorts,
		VlanId:            p.VlanID,
		NetworkType:       p.NetworkType,
		EnableOutboundNat: true,
	}
	if strings.EqualFold(strings.TrimSpace(p.NetworkType), "vxlan") {
		req.EnableOutboundNat = p.EnableOutboundNat
	}
	resp, err := e.api.CreateNetwork(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, p.Name)
	cr.Status.AtProvider.NodeID = resp.GetNodeId()
	cr.Status.AtProvider.Message = resp.GetMessage()
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *apisv1alpha1.Network) (managed.ExternalUpdate, error) {
	// Networks are typically replaced by delete+create in kcore; no Update RPC.
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *apisv1alpha1.Network) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	name := netName(cr)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	_, err := e.api.DeleteNetwork(ctx, &kcorepb.DeleteNetworkRequest{
		Name:       name,
		TargetNode: strings.TrimSpace(cr.Spec.ForProvider.TargetNode),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, err
	}
	return managed.ExternalDelete{}, nil
}
