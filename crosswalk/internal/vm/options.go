package vm

import (
	"go-deployer/internal/config"
	"go-deployer/internal/scp"
)

type RunOptions struct {
	DryRun          bool
	BastionOnly     bool
	ServerTags      []string
	AppTags         []string
	ParallelServers int
	TransferSession *scp.TransferSession
	DirectoryRules  []config.DirectoryOwnershipRule
	outcomes        *operationOutcomes
}
