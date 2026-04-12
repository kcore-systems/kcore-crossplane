package securitygroup

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

// SetupGated registers the SecurityGroup controller.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup SecurityGroup controller"))
		}
	}, apisv1alpha1.SecurityGroupGroupVersionKind)
	return nil
}

// Setup adds the reconciler.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(apisv1alpha1.SecurityGroupGroupKind)
	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*apisv1alpha1.SecurityGroup](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &apisv1alpha1.SecurityGroupList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(rec); err != nil {
			return errors.Wrap(err, "register MR state metrics")
		}
	}
	r := managed.NewReconciler(mgr, resource.ManagedKind(apisv1alpha1.SecurityGroupGroupVersionKind), opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&apisv1alpha1.SecurityGroup{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *apisv1alpha1.SecurityGroup) (managed.TypedExternalClient[*apisv1alpha1.SecurityGroup], error) {
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

func sgName(cr *apisv1alpha1.SecurityGroup) string {
	if en := meta.GetExternalName(cr); en != "" {
		return en
	}
	return strings.TrimSpace(cr.Spec.ForProvider.Name)
}

func rulesProto(rules []apisv1alpha1.SecurityGroupRule) []*kcorepb.SecurityGroupRule {
	out := make([]*kcorepb.SecurityGroupRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, &kcorepb.SecurityGroupRule{
			Id:         r.ID,
			Protocol:   r.Protocol,
			HostPort:   r.HostPort,
			TargetPort: r.TargetPort,
			SourceCidr: r.SourceCidr,
			TargetVm:   r.TargetVM,
			EnableDnat: r.EnableDnat,
		})
	}
	return out
}

func (e *external) Observe(ctx context.Context, cr *apisv1alpha1.SecurityGroup) (managed.ExternalObservation, error) {
	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	name := sgName(cr)
	if name == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	resp, err := e.api.GetSecurityGroup(ctx, &kcorepb.GetSecurityGroupRequest{Name: name})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}
	sg := resp.GetSecurityGroup()
	if sg == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	cr.Status.AtProvider.Name = sg.GetName()
	// Controller has no UpdateSecurityGroup; treat as up-to-date when present.
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, cr *apisv1alpha1.SecurityGroup) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	p := cr.Spec.ForProvider
	if strings.TrimSpace(p.Name) == "" {
		return managed.ExternalCreation{}, errors.New("spec.forProvider.name is required")
	}
	_, err := e.api.CreateSecurityGroup(ctx, &kcorepb.CreateSecurityGroupRequest{
		SecurityGroup: &kcorepb.SecurityGroup{
			Name:        p.Name,
			Description: p.Description,
			Rules:       rulesProto(p.Rules),
		},
	})
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, p.Name)
	cr.Status.AtProvider.Name = p.Name
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *apisv1alpha1.SecurityGroup) (managed.ExternalUpdate, error) {
	// No UpdateSecurityGroup RPC — delete/recreate is operator-driven.
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *apisv1alpha1.SecurityGroup) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	name := sgName(cr)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	_, err := e.api.DeleteSecurityGroup(ctx, &kcorepb.DeleteSecurityGroupRequest{Name: name})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, err
	}
	return managed.ExternalDelete{}, nil
}
