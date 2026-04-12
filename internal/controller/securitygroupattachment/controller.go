package securitygroupattachment

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

// SetupGated registers the SecurityGroupAttachment controller.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup SecurityGroupAttachment controller"))
		}
	}, apisv1alpha1.SecurityGroupAttachmentGroupVersionKind)
	return nil
}

// Setup adds the reconciler.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(apisv1alpha1.SecurityGroupAttachmentGroupKind)
	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*apisv1alpha1.SecurityGroupAttachment](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &apisv1alpha1.SecurityGroupAttachmentList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(rec); err != nil {
			return errors.Wrap(err, "register MR state metrics")
		}
	}
	r := managed.NewReconciler(mgr, resource.ManagedKind(apisv1alpha1.SecurityGroupAttachmentGroupVersionKind), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&apisv1alpha1.SecurityGroupAttachment{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *apisv1alpha1.SecurityGroupAttachment) (managed.TypedExternalClient[*apisv1alpha1.SecurityGroupAttachment], error) {
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

func attachmentKey(cr *apisv1alpha1.SecurityGroupAttachment) string {
	if en := meta.GetExternalName(cr); en != "" {
		return en
	}
	p := cr.Spec.ForProvider
	return strings.Join([]string{
		strings.TrimSpace(p.SecurityGroup),
		strings.ToLower(strings.TrimSpace(p.TargetKind)),
		strings.TrimSpace(p.TargetID),
	}, "/")
}

func (e *external) findAttachment(ctx context.Context, cr *apisv1alpha1.SecurityGroupAttachment) (bool, error) {
	p := cr.Spec.ForProvider
	sgName := strings.TrimSpace(p.SecurityGroup)
	if sgName == "" {
		return false, errors.New("spec.forProvider.securityGroup is required")
	}
	resp, err := e.api.GetSecurityGroup(ctx, &kcorepb.GetSecurityGroupRequest{Name: sgName})
	if err != nil {
		return false, err
	}
	wantKind := kcore.SecurityGroupTargetKind(p.TargetKind)
	wantID := strings.TrimSpace(p.TargetID)
	for _, a := range resp.GetAttachments() {
		if a.GetSecurityGroup() == sgName && a.GetTargetKind() == wantKind && a.GetTargetId() == wantID {
			return true, nil
		}
	}
	return false, nil
}

func (e *external) Observe(ctx context.Context, cr *apisv1alpha1.SecurityGroupAttachment) (managed.ExternalObservation, error) {
	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if attachmentKey(cr) == "//" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	ok, err := e.findAttachment(ctx, cr)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}
	if !ok {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	cr.Status.AtProvider.Attached = true
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, cr *apisv1alpha1.SecurityGroupAttachment) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	p := cr.Spec.ForProvider
	_, err := e.api.AttachSecurityGroup(ctx, &kcorepb.AttachSecurityGroupRequest{
		SecurityGroup: strings.TrimSpace(p.SecurityGroup),
		TargetKind:    kcore.SecurityGroupTargetKind(p.TargetKind),
		TargetId:      strings.TrimSpace(p.TargetID),
		TargetNode:    strings.TrimSpace(p.TargetNode),
	})
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, attachmentKey(cr))
	cr.Status.AtProvider.Attached = true
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *apisv1alpha1.SecurityGroupAttachment) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *apisv1alpha1.SecurityGroupAttachment) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	p := cr.Spec.ForProvider
	_, err := e.api.DetachSecurityGroup(ctx, &kcorepb.DetachSecurityGroupRequest{
		SecurityGroup: strings.TrimSpace(p.SecurityGroup),
		TargetKind:    kcore.SecurityGroupTargetKind(p.TargetKind),
		TargetId:      strings.TrimSpace(p.TargetID),
		TargetNode:    strings.TrimSpace(p.TargetNode),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, err
	}
	return managed.ExternalDelete{}, nil
}
