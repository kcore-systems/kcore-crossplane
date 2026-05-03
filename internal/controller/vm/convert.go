package vm

import (
	"strings"

	apisv1alpha1 "github.com/kcore/kcore-crossplane/apis/kcore/v1alpha1"
	kcorepb "github.com/kcore/kcore-crossplane/gen/proto/kcore/controller/v1"
	"github.com/kcore/kcore-crossplane/internal/controller/kcore"
)

// BuildVmSpec builds a proto VmSpec from VirtualMachineParameters.
func BuildVmSpec(p apisv1alpha1.VirtualMachineParameters) *kcorepb.VmSpec {
	disks := make([]*kcorepb.Disk, 0, len(p.Disks))
	for _, d := range p.Disks {
		disks = append(disks, &kcorepb.Disk{
			Name:          d.Name,
			BackendHandle: d.BackendHandle,
			Bus:           d.Bus,
			Device:        d.Device,
		})
	}
	nics := make([]*kcorepb.Nic, 0, len(p.Nics))
	for _, n := range p.Nics {
		nics = append(nics, &kcorepb.Nic{
			Network:    n.Network,
			Model:      n.Model,
			MacAddress: n.MacAddress,
		})
	}
	return &kcorepb.VmSpec{
		Name:             p.Name,
		Cpu:              p.CPUs,
		MemoryBytes:      p.MemoryBytes,
		Disks:            disks,
		Nics:             nics,
		StorageBackend:   strings.TrimSpace(p.StorageBackend),
		StorageSizeBytes: p.StorageSizeBytes,
		DesiredState:     kcore.VmDesiredState(p.DesiredState),
	}
}

func createVMRequest(p apisv1alpha1.VirtualMachineParameters) *kcorepb.CreateVmRequest {
	return &kcorepb.CreateVmRequest{
		TargetNode:        p.TargetNode,
		Spec:              BuildVmSpec(p),
		ImageUrl:          p.ImageURL,
		ImageSha256:       p.ImageSha256,
		CloudInitUserData: p.CloudInit,
		ImagePath:         p.ImagePath,
		ImageFormat:       p.ImageFormat,
		SshKeyNames:       p.SSHKeyNames,
		StorageBackend:    kcore.StorageBackend(p.StorageBackend),
		StorageSizeBytes:  p.StorageSizeBytes,
		TargetDc:          strings.TrimSpace(p.TargetDc),
	}
}
