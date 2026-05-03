package kcore

import (
	"strings"

	kcorepb "github.com/kcore/kcore-crossplane/gen/proto/kcore/controller/v1"
)

// StorageBackend maps user strings to proto enum.
func StorageBackend(s string) kcorepb.StorageBackendType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "unspecified":
		return kcorepb.StorageBackendType_STORAGE_BACKEND_TYPE_UNSPECIFIED
	case "filesystem", "file":
		return kcorepb.StorageBackendType_STORAGE_BACKEND_TYPE_FILESYSTEM
	case "lvm":
		return kcorepb.StorageBackendType_STORAGE_BACKEND_TYPE_LVM
	case "zfs":
		return kcorepb.StorageBackendType_STORAGE_BACKEND_TYPE_ZFS
	default:
		return kcorepb.StorageBackendType_STORAGE_BACKEND_TYPE_UNSPECIFIED
	}
}

// WorkloadKind parses "vm" / "container".
func WorkloadKind(s string) kcorepb.WorkloadKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "vm", "virtualmachine":
		return kcorepb.WorkloadKind_WORKLOAD_KIND_VM
	case "container":
		return kcorepb.WorkloadKind_WORKLOAD_KIND_CONTAINER
	default:
		return kcorepb.WorkloadKind_WORKLOAD_KIND_UNSPECIFIED
	}
}

// VmDesiredState parses declarative VM desired state (running / stopped).
func VmDesiredState(s string) kcorepb.VmDesiredState {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "unspecified":
		return kcorepb.VmDesiredState_VM_DESIRED_STATE_UNSPECIFIED
	case "running", "run":
		return kcorepb.VmDesiredState_VM_DESIRED_STATE_RUNNING
	case "stopped", "stop":
		return kcorepb.VmDesiredState_VM_DESIRED_STATE_STOPPED
	default:
		return kcorepb.VmDesiredState_VM_DESIRED_STATE_UNSPECIFIED
	}
}

// WorkloadDesiredState parses container/workload declarative state.
func WorkloadDesiredState(s string) kcorepb.WorkloadDesiredState {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "unspecified":
		return kcorepb.WorkloadDesiredState_WORKLOAD_DESIRED_STATE_UNSPECIFIED
	case "running", "run":
		return kcorepb.WorkloadDesiredState_WORKLOAD_DESIRED_STATE_RUNNING
	case "stopped", "stop":
		return kcorepb.WorkloadDesiredState_WORKLOAD_DESIRED_STATE_STOPPED
	default:
		return kcorepb.WorkloadDesiredState_WORKLOAD_DESIRED_STATE_UNSPECIFIED
	}
}

// SecurityGroupTargetKind parses "vm" / "network".
func SecurityGroupTargetKind(s string) kcorepb.SecurityGroupTargetKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "vm", "virtualmachine":
		return kcorepb.SecurityGroupTargetKind_SECURITY_GROUP_TARGET_KIND_VM
	case "network":
		return kcorepb.SecurityGroupTargetKind_SECURITY_GROUP_TARGET_KIND_NETWORK
	default:
		return kcorepb.SecurityGroupTargetKind_SECURITY_GROUP_TARGET_KIND_UNSPECIFIED
	}
}
