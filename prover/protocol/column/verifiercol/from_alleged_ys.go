package verifiercol

import (
	"strings"

	"github.com/consensys/accelerated-crypto-monorepo/maths/common/smartvectors"
	"github.com/consensys/accelerated-crypto-monorepo/maths/field"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/ifaces"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/query"
	"github.com/consensys/accelerated-crypto-monorepo/protocol/wizard"
	"github.com/consensys/gnark/frontend"
	"github.com/sirupsen/logrus"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = FromYs{}

// Represents a column populated by alleged evaluations of arrange of columns
type FromYs struct {
	// The list of the evaluated column in the same order
	// as we like to layout the currently-described column
	ranges []ifaces.ColID
	// The query from which we shall select the evaluations
	query query.UnivariateEval
	// Remember the round in which the query was made
	round int
}

// Construct a new column from a univariate query and a list of of ifaces.ColID
// If passed a column that is not part of the query. It will not panic but it will
// return a zero entry. This is the expected behavior when given a shadow column
// from the vortex compiler but otherwise this is a bug.
func NewFromYs(comp *wizard.CompiledIOP, q query.UnivariateEval, ranges []ifaces.ColID) ifaces.Column {

	// All the names in the range should also be part of the query.
	// To make sure of this, we build the following map.
	nameMap := map[ifaces.ColID]struct{}{}
	for _, polName := range q.Pols {
		nameMap[polName.GetColID()] = struct{}{}
	}

	// No make the explicit check
	for _, rangeName := range ranges {
		if _, ok := nameMap[rangeName]; !ok && !strings.Contains(string(rangeName), "SHADOW") {
			logrus.Errorf("NewFromYs : %v is not part of the query %v. It will be zeroized", rangeName, q.QueryID)
		}
	}

	// Make sure that the query is indeed registered in the current wizard.
	comp.QueriesParams.MustExists(q.QueryID)
	round := comp.QueriesParams.Round(q.QueryID)

	res := FromYs{
		ranges: ranges,
		query:  q,
		round:  round,
	}

	return res
}

// Returns the round of definition of the column
func (fys FromYs) Round() int {
	return fys.round
}

// Returns a generic name from the column. Defined from the coin's.
func (fys FromYs) GetColID() ifaces.ColID {
	return ifaces.ColIDf("FYS_%v", fys.query.QueryID)
}

// Always return true. We sanity-check the existence of the
// random coin prior to constructing the object.
func (fys FromYs) MustExists() {}

// Return the size of the fys
func (fys FromYs) Size() int {
	return len(fys.ranges)
}

// Returns the coin's value as a column assignment
func (fys FromYs) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {

	queryParams := run.GetParams(fys.query.QueryID).(query.UnivariateEvalParams)

	// Map the alleged evaluations to their respective commitment names
	yMap := map[ifaces.ColID]field.Element{}
	for i, polName := range fys.query.Pols {
		yMap[polName.GetColID()] = queryParams.Ys[i]
	}

	// This will leaves the columns missing from the query to zero.
	res := make([]field.Element, len(fys.ranges))
	for i, name := range fys.ranges {
		res[i] = yMap[name]
	}

	return smartvectors.NewRegular(res)
}

// Returns the coin's value as a column assignment
func (fys FromYs) GetColAssignmentGnark(run ifaces.GnarkRuntime) []frontend.Variable {

	queryParams := run.GetParams(fys.query.QueryID).(query.GnarkUnivariateEvalParams)

	// Map the alleged evaluations to their respective commitment names
	yMap := map[ifaces.ColID]frontend.Variable{}
	for i, polName := range fys.query.Pols {
		yMap[polName.GetColID()] = queryParams.Ys[i]
	}

	// This will leave some of the columns to nil
	res := make([]frontend.Variable, len(fys.ranges))
	for i, name := range fys.ranges {
		if y, found := yMap[name]; found {
			res[i] = y
		} else {
			// Set it to zero explicitly
			res[i] = frontend.Variable(0)
		}
	}

	return res
}

// Returns a particular position of the coin value
func (fys FromYs) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	return fys.GetColAssignment(run).Get(pos)
}

// Returns a particular position of the coin value
func (fys FromYs) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) frontend.Variable {
	return fys.GetColAssignmentGnark(run)[pos]
}

func (fys FromYs) IsComposite() bool {
	return false
}

// Returns the name of the column.
func (fys FromYs) String() string {
	return string(fys.GetColID())
}

// Split the FromYs by restricting to a range
func (fys FromYs) Split(comp *wizard.CompiledIOP, from, to int) ifaces.Column {
	return NewFromYs(comp, fys.query, fys.ranges[from:to])
}
