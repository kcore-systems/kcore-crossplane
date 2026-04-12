package workload

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
	"github.com/kcore/kcore-crossplane/internal/controller/vm"
)

// SetupGated registers the Workload controller.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Workload controller"))
		}
	}, apisv1alpha1.WorkloadGroupVersionKind)
	return nil
}

// Setup adds the reconciler.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(apisv1alpha1.WorkloadGroupKind)
	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*apisv1alpha1.Workload](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &apisv1alpha1.WorkloadList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(rec); err != nil {
			return errors.Wrap(err, "register MR state metrics")
		}
	}
	r := managed.NewReconciler(mgr, resource.ManagedKind(apisv1alpha1.WorkloadGroupVersionKind), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&apisv1alpha1.Workload{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *apisv1alpha1.Workload) (managed.TypedExternalClient[*apisv1alpha1.Workload], error) {
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

func wlID(cr *apisv1alpha1.Workload) string {
	return meta.GetExternalName(cr)
}

func containerProto(c apisv1alpha1.ContainerParameters) *kcorepb.ContainerSpec {
	if strings.TrimSpace(c.Name) == "" && c.Image == "" {
		return nil
	}
	return &kcorepb.ContainerSpec{
		Name:             c.Name,
		Image:            c.Image,
		Network:          c.Network,
		Command:          c.Command,
		Env:              c.Env,
		Ports:            c.Ports,
		StorageBackend:   c.StorageBackend,
		StorageSizeBytes: c.StorageSizeBytes,
		MountTarget:      c.MountTarget,
	}
}

func (e *external) Observe(ctx context.Context, cr *apisv1alpha1.Workload) (managed.ExternalObservation, error) {
	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	id := wlID(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	kind := kcore.WorkloadKind(cr.Spec.ForProvider.Kind)
	if kind == kcorepb.WorkloadKind_WORKLOAD_KIND_UNSPECIFIED {
		return managed.ExternalObservation{}, errors.New("invalid spec.forProvider.kind")
	}
	resp, err := e.api.GetWorkload(ctx, &kcorepb.GetWorkloadRequest{
		Kind:       kind,
		WorkloadId: id,
		TargetNode: strings.TrimSpace(cr.Spec.ForProvider.TargetNode),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}
	cr.Status.AtProvider.WorkloadID = id
	cr.Status.AtProvider.NodeID = resp.GetNodeId()
	cr.Status.AtProvider.AssignedIP = resp.GetAssignedIp()
	if vmst := resp.GetVmStatus(); vmst != nil {
		cr.Status.AtProvider.VMState = vmst.GetState().String()
	}
	if ci := resp.GetContainerInfo(); ci != nil {
		cr.Status.AtProvider.ContainerState = ci.GetState().String()
	}
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, cr *apisv1alpha1.Workload) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	p := cr.Spec.ForProvider
	kind := kcore.WorkloadKind(p.Kind)
	if kind == kcorepb.WorkloadKind_WORKLOAD_KIND_UNSPECIFIED {
		return managed.ExternalCreation{}, errors.New("spec.forProvider.kind must be vm or container")
	}
	if p.StorageSizeBytes <= 0 {
		return managed.ExternalCreation{}, errors.New("spec.forProvider.storageSizeBytes must be > 0")
	}
	req := &kcorepb.CreateWorkloadRequest{
		Kind:              kind,
		TargetNode:        p.TargetNode,
		ImageUrl:          p.ImageURL,
		ImageSha256:       p.ImageSha256,
		CloudInitUserData: p.CloudInit,
		ImagePath:         p.ImagePath,
		ImageFormat:       p.ImageFormat,
		SshKeyNames:       p.SSHKeyNames,
		StorageBackend:    kcore.StorageBackend(p.StorageBackend),
		StorageSizeBytes:  p.StorageSizeBytes,
	}
	if kind == kcorepb.WorkloadKind_WORKLOAD_KIND_VM {
		req.VmSpec = vm.BuildVmSpec(p.VM)
	} else {
		req.ContainerSpec = containerProto(p.Container)
	}
	resp, err := e.api.CreateWorkload(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, resp.GetWorkloadId())
	cr.Status.AtProvider.WorkloadID = resp.GetWorkloadId()
	cr.Status.AtProvider.NodeID = resp.GetNodeId()
	cr.Status.AtProvider.VMState = resp.GetVmState().String()
	cr.Status.AtProvider.ContainerState = resp.GetContainerState().String()
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *apisv1alpha1.Workload) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *apisv1alpha1.Workload) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	id := wlID(cr)
	if id == "" {
		return managed.ExternalDelete{}, nil
	}
	kind := kcore.WorkloadKind(cr.Spec.ForProvider.Kind)
	_, err := e.api.DeleteWorkload(ctx, &kcorepb.DeleteWorkloadRequest{
		Kind:       kind,
		WorkloadId: id,
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
