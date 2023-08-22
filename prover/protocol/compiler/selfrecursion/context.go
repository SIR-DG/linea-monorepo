package selfrecursion

import (
	"github.com/consensys/accelerated-crypto-monorepo/crypto/ringsis"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/coin"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/column"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/compiler/vortex"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/ifaces"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/wizard"
	"github.com/consensys/accelerated-crypto-monorepo/utils"
)

// Defines a context of self-recursion
type SelfRecursionCtx struct {

	// Pointer to the compiled IOP
	comp *wizard.CompiledIOP

	// Context of the past vortex compiler that we are skipping
	VortexCtx *vortex.Ctx

	// Snapshot of the self-recursion counter. Contains the informa
	// of how many time self-recursion was applied prior to the current
	// step.
	SelfRecursionCnt int

	// Accessors
	Accessors struct {
		// (EvalBivariate)
		//
		// Bivariate evaluation of the collapsed preimage by (2, \alpha)
		BivariateEvalPreimagesCollapsed *ifaces.Accessor

		// (CoeffUnivariate)
		//
		// Univariate evaluation of UalphaQ by (rCollapse)
		CoeffEvalUalphaCollapse *ifaces.Accessor

		// (InterpolationUnivariate)
		//
		// Interpolation evaluation of Ualpha by X
		InterpolateUalphaX *ifaces.Accessor

		// (CoeffUnivariate)
		//
		// Coefficient evaluation of Ys by Alpha
		CoeffEvalYsAlpha *ifaces.Accessor
	}

	// List all the queries happening within the subprotocol
	// (at the highest level)
	Queries struct {
		// (RANGE)
		//
		// Checks the shortness of the preimages
		PreimagesAreShort []ifaces.Query

		// (INNER-PRODUCT)
		//
		// Computes the inner-product of the collapsed+folded preimages
		// with the folded+merged SIS key
		LatticeInnerProd ifaces.Query
	}

	// List all the coins happening within the subprotocol
	Coins struct {
		// X, the evaluation point of the claim
		X ifaces.Accessor

		// Alpha, accessor to the evaluation
		Alpha coin.Info

		// Merge coin, combine the separate sis evaluations into one
		Merge coin.Info

		// Squash, used to fold the sis hashes into a single value for
		// the selection phase
		Squash coin.Info

		// Q, the columns selected by the verifier
		Q coin.Info

		// Collapse, used to batch the opened SIS preimages into a single
		// one.
		Collapse coin.Info

		// Fold, used to evaluate the collapse the SIS ring elements into
		// a normal single inner-product
		Fold coin.Info
	}

	// List all the columns interacting with the current protocol
	Columns struct {
		// (Precomputed)
		//
		// I(X) interpolates (0, 1, 2, 3, 4, ...., nb encoded columns)
		I ifaces.Column

		// (Precomputed)
		//
		// SIS key chunks indexed by rounds. Since some rounds may be
		// dried some of the Ah are `nil`
		Ah []ifaces.Column

		// (Commitments round-by-round, already computed)
		//
		// Round-by-rounds commitments. Since some rounds may be dried
		// some of the RoundDigest can be `nil`
		Dh []ifaces.Column

		// (Verifier column)
		//
		// Gathers the claimed evaluations to be proven
		Ys ifaces.Column

		// (Commitment, already computed)
		//
		// LinearCombination claimed by the verifier
		Ualpha ifaces.Column

		// (Proof, already computed)
		//
		// Preimages of the selected columns in whole form. Is set to be
		// ignored by the self-recursion compiler. Implictly it is repla-
		// ced by the `Preimages`, which contains the preimages but in
		// limb expanded form.
		WholePreimages []ifaces.Column

		// (Commitments, to compute)
		//
		// Preimages of the columns (in limb form) that have been opened.
		// This columns are concatenated "round-by-round" and the concate-
		// nation is zero-padded at the end to the next power of two
		Preimages []ifaces.Column

		// (Auto-computed)
		//
		// Dmerge represents the linear combination of the digests using
		// the rMerge coin
		Dmerge ifaces.Column

		// (Auto-computed)
		//
		// Amerge represents the linear combination of the key chunks and
		// represents the merged key
		Amerge ifaces.Column

		// (Verifier column)
		//
		// Gathers the selected columns to open
		Q ifaces.Column

		// (Commitment, to compute)
		//
		// Represents the merged digest entries selected by Q
		DmergeQ ifaces.Column

		// (Commitment, to compute)
		//
		// Represents the entries of the linear combination Ualpha selected
		// by Q
		UalphaQ ifaces.Column

		// (Auto-computed)
		//
		// Represents the Dmergeq folded by rSquash
		DmergeQsquash ifaces.Column

		// (Auto-computed)
		//
		// Represents Dmerge folded by rSquash
		DmergeSquash ifaces.Column

		// (Auto-computed)
		//
		// The linear combination of the preimages by rCollapse
		PreimagesCollapse ifaces.Column

		// (Auto-computed)
		//
		// The folding of DmergeQ by rCollapse
		DmergeQcollapse ifaces.Column

		// (To evaluate)
		//
		// The dual sis hash of the collapsed preimage by Amerge
		Edual ifaces.Column

		// (Auto-computed)
		//
		// The folding of Amerge by rFold
		AmergeFold ifaces.Column

		// (Auto-computed)
		//
		// The folding of preimageCollapse by rFold
		PreimageCollapseFold ifaces.Column
	}
}

// Initializes a context for the self recursion
func NewSelfRecursionCxt(comp *wizard.CompiledIOP) SelfRecursionCtx {

	// Extract the vortex context from the compiledIOP though
	// the "CryptographicCompilerCtx"
	vortexCtx := assertVortexCompiled(comp)

	ctx := SelfRecursionCtx{
		comp:             comp,
		VortexCtx:        vortexCtx,
		SelfRecursionCnt: comp.SelfRecursionCount,
	}

	// Assert that the self-recursion transformation
	// operates over a merkle-mode vortex compiled
	// wizard
	if ctx.VortexCtx.UseMerkleProof {
		panic("expected self-recursion not in merkle-mode")
	}

	// Transport the compilation items of the vortex context into
	// the new self-recursion context
	ctx.Columns.Dh = vortexCtx.Items.Dh
	ctx.Coins.Alpha = vortexCtx.Items.Alpha
	ctx.Columns.Ualpha = vortexCtx.Items.Ualpha
	ctx.Coins.Q = vortexCtx.Items.Q
	ctx.Columns.WholePreimages = vortexCtx.Items.OpenedColumns

	// Mark all the rounds commitments `Dh` (assumedly of status
	// `Proof`) as `Committed`
	for _, dh := range ctx.Columns.Dh {

		if dh == nil {
			// Skip it, it is a dry round
			continue
		}

		// Assume that the rounds commitments have a `Proof` status
		if comp.Columns.Status(dh.GetColID()) != column.Proof {
			utils.Panic(
				"Assumed the Dh to be %v but status is %v",
				column.Proof.String(),
				comp.Columns.Status(dh.GetColID()),
			)
		}
		comp.Columns.SetStatus(dh.GetColID(), column.Committed)
	}

	// Likewise, assume that Ualpha has a status of `Proof` and then
	// mark it as a `Committed`
	if comp.Columns.Status(ctx.Columns.Ualpha.GetColID()) != column.Proof {
		utils.Panic(
			"Assumed Ualpha to be %v but status is %v",
			column.Proof.String(),
			comp.Columns.Status(ctx.Columns.Ualpha.GetColID()).String(),
		)
	}
	comp.Columns.SetStatus(ctx.Columns.Ualpha.GetColID(), column.Committed)

	// And for the `WholePreimage`, we mark it as `Ignored` and make the
	// same assumption that theirs status is `Proof`
	for _, opened := range ctx.Columns.WholePreimages {
		// Assume that the rounds commitments have a `Proof` status
		if comp.Columns.Status(opened.GetColID()) != column.Proof {
			utils.Panic(
				"Assumed the Dh to be %v but status is %v",
				column.Proof.String(),
				comp.Columns.Status(opened.GetColID()),
			)
		}
		comp.Columns.SetStatus(opened.GetColID(), column.Ignored)
	}

	return ctx
}

// Asserts that the compiled IOP has the appropriate cryptographic context
func assertVortexCompiled(comp *wizard.CompiledIOP) *vortex.Ctx {
	// When we compiled using Vortex, we annotated the compiledIOP
	// that the current protocol was a result of the
	ctx := comp.CryptographicCompilerCtx

	// Check for non-nilness
	if ctx == nil {
		panic("nil cryptographic compiler context")
	}

	// Check for the correct type
	if _, ok := ctx.(*vortex.Ctx); !ok {
		utils.Panic("Not the correct type %T", ctx)
	}

	vortexCtx := ctx.(*vortex.Ctx)
	// Also "stamp" that the compilation context has been cancelled
	// this means that the verifier part of vortex will be ignored
	// (and will be replaced by what is declared in the self-recursion)
	// The "Dried" part of the vortex compiler is NOT ignored though.
	vortexCtx.IsSelfrecursed = true

	return ctx.(*vortex.Ctx)
}

// Accessor for the SIS key
func (ctx *SelfRecursionCtx) SisKey() *ringsis.Key {
	return &ctx.VortexCtx.VortexParams.Key
}
