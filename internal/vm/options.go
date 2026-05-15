package vm

import "go-deployer/internal/scp"

type RunOptions struct {
	DryRun          bool
	ServerTags      []string
	AppTags         []string
	ParallelServers int
	TransferSession *scp.TransferSession
}
