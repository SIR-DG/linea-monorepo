package define

import "github.com/consensys/zkevm-monorepo/prover/config"

// Settings specifies the parameters for the arithmetization part of the
// zkEVM.
type Settings struct {
	// Configuration object specifying the columns limits
	Traces        *config.TracesLimits
	ColDepthLimit int
	NumColLimit   int
}
