package config

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"

	apisv1alpha1 "github.com/kcore/kcore-crossplane/apis/kcore/v1alpha1"
)

// Setup reconciles ProviderConfig / ClusterProviderConfig usage.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if err := setupNamespaced(mgr, o); err != nil {
		return err
	}
	return setupCluster(mgr, o)
}

func setupNamespaced(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apisv1alpha1.ProviderConfigGroupKind)
	of := resource.ProviderConfigKinds{
		Config:    apisv1alpha1.ProviderConfigGroupVersionKind,
		Usage:     apisv1alpha1.ProviderConfigUsageGroupVersionKind,
		UsageList: apisv1alpha1.ProviderConfigUsageListGroupVersionKind,
	}
	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apisv1alpha1.ProviderConfig{}).
		Watches(&apisv1alpha1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

func setupCluster(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apisv1alpha1.ClusterProviderConfigGroupKind)
	of := resource.ProviderConfigKinds{
		Config:    apisv1alpha1.ClusterProviderConfigGroupVersionKind,
		Usage:     apisv1alpha1.ClusterProviderConfigUsageGroupVersionKind,
		UsageList: apisv1alpha1.ClusterProviderConfigUsageListGroupVersionKind,
	}
	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apisv1alpha1.ClusterProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}
