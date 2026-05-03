package vm

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

// SetupGated registers the VirtualMachine controller.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup VirtualMachine controller"))
		}
	}, apisv1alpha1.VirtualMachineGroupVersionKind)
	return nil
}

// Setup adds the reconciler.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(apisv1alpha1.VirtualMachineGroupKind)
	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*apisv1alpha1.VirtualMachine](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &apisv1alpha1.VirtualMachineList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(rec); err != nil {
			return errors.Wrap(err, "register MR state metrics")
		}
	}
	r := managed.NewReconciler(mgr, resource.ManagedKind(apisv1alpha1.VirtualMachineGroupVersionKind), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&apisv1alpha1.VirtualMachine{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *apisv1alpha1.VirtualMachine) (managed.TypedExternalClient[*apisv1alpha1.VirtualMachine], error) {
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

func vmExternalID(cr *apisv1alpha1.VirtualMachine) string {
	if en := meta.GetExternalName(cr); en != "" {
		return en
	}
	return ""
}

func (e *external) Observe(ctx context.Context, cr *apisv1alpha1.VirtualMachine) (managed.ExternalObservation, error) {
	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	id := vmExternalID(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	resp, err := e.api.GetVm(ctx, &kcorepb.GetVmRequest{
		VmId:       id,
		TargetNode: strings.TrimSpace(cr.Spec.ForProvider.TargetNode),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}
	spec := resp.GetSpec()
	st := resp.GetStatus()
	cr.Status.AtProvider.VMID = id
	cr.Status.AtProvider.NodeID = resp.GetNodeId()
	cr.Status.AtProvider.AssignedIP = resp.GetAssignedIp()
	if st != nil {
		cr.Status.AtProvider.State = st.GetState().String()
	}
	if spec != nil {
		cr.Status.AtProvider.StorageBackend = spec.GetStorageBackend()
		cr.Status.AtProvider.StorageSizeBytes = spec.GetStorageSizeBytes()
		cr.Status.AtProvider.DesiredState = spec.GetDesiredState().String()
		wantDs := kcore.VmDesiredState(cr.Spec.ForProvider.DesiredState)
		dsOK := cr.Spec.ForProvider.DesiredState == "" || spec.GetDesiredState() == wantDs
		upToDate := spec.GetCpu() == cr.Spec.ForProvider.CPUs &&
			spec.GetMemoryBytes() == cr.Spec.ForProvider.MemoryBytes && dsOK
		cr.Status.SetConditions(xpv1.Available())
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: upToDate,
		}, nil
	}
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, cr *apisv1alpha1.VirtualMachine) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	p := cr.Spec.ForProvider
	if strings.TrimSpace(p.Name) == "" {
		return managed.ExternalCreation{}, errors.New("spec.forProvider.name is required")
	}
	if p.StorageSizeBytes <= 0 {
		return managed.ExternalCreation{}, errors.New("spec.forProvider.storageSizeBytes must be > 0")
	}
	resp, err := e.api.CreateVm(ctx, createVMRequest(p))
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, resp.GetVmId())
	cr.Status.AtProvider.VMID = resp.GetVmId()
	cr.Status.AtProvider.NodeID = resp.GetNodeId()
	cr.Status.AtProvider.State = resp.GetState().String()
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *apisv1alpha1.VirtualMachine) (managed.ExternalUpdate, error) {
	id := vmExternalID(cr)
	if id == "" {
		return managed.ExternalUpdate{}, nil
	}
	_, err := e.api.UpdateVm(ctx, &kcorepb.UpdateVmRequest{
		VmId:        id,
		TargetNode:  strings.TrimSpace(cr.Spec.ForProvider.TargetNode),
		Cpu:         cr.Spec.ForProvider.CPUs,
		MemoryBytes: cr.Spec.ForProvider.MemoryBytes,
	})
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *apisv1alpha1.VirtualMachine) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	id := vmExternalID(cr)
	if id == "" {
		return managed.ExternalDelete{}, nil
	}
	_, err := e.api.DeleteVm(ctx, &kcorepb.DeleteVmRequest{
		VmId:       id,
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
