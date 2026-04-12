package controller

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kcore/kcore-crossplane/internal/controller/config"
	"github.com/kcore/kcore-crossplane/internal/controller/network"
	"github.com/kcore/kcore-crossplane/internal/controller/securitygroup"
	"github.com/kcore/kcore-crossplane/internal/controller/securitygroupattachment"
	"github.com/kcore/kcore-crossplane/internal/controller/sshkey"
	"github.com/kcore/kcore-crossplane/internal/controller/vm"
	"github.com/kcore/kcore-crossplane/internal/controller/workload"
)

// SetupGated wires all reconcilers (including ProviderConfig).
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		config.Setup,
		sshkey.SetupGated,
		network.SetupGated,
		securitygroup.SetupGated,
		securitygroupattachment.SetupGated,
		vm.SetupGated,
		workload.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
