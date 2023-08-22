package test_cases_test

import (
	"testing"

	"github.com/consensys/accelerated-crypto-monorepo/crypto/mimc/gkrmimc"
	"github.com/consensys/accelerated-crypto-monorepo/maths/common/smartvectors"
	"github.com/consensys/accelerated-crypto-monorepo/maths/field"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/column"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/compiler/splitter"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/wizard"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

func defineLocalOpening(builder *wizard.Builder) {
	P1 := builder.RegisterCommit(P1, 8)
	P1Next := column.Shift(P1, 1)
	P2Prev := column.Shift(P1, -1)
	builder.LocalOpening("Inclusion1", P1Next)
	builder.LocalOpening("Inclusion2", P2Prev)
}

func proverLocalOpening(run *wizard.ProverRuntime) {
	run.AssignColumn(P1, smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7))
	run.AssignLocalPoint("Inclusion1", field.NewElement(1))
	run.AssignLocalPoint("Inclusion2", field.NewElement(7))
}

func TestGnarkCompile(t *testing.T) {
	comp := wizard.Compile(defineLocalOpening, splitter.SplitColumns(32))
	proof := wizard.Prove(comp, proverLocalOpening)

	circ, err := wizard.AllocateWizardCircuit(comp)
	if err != nil {
		panic(err)
	}

	scs, err := frontend.Compile(field.Modulus(), scs.NewBuilder, &SimpleTestGnarkCircuit{C: *circ})
	if err != nil {
		panic(err)
	}

	assignment := GetAssignment(comp, proof)
	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	require.NoError(t, err)

	err = scs.IsSolved(witness, gkrmimc.SolverOpts(scs)...)
	if err != nil {
		// When the error string is too large `require.NoError` does not print
		// the error.
		t.Fatalf("circuit solving failed : %v\n", err)
	}
}
