package eth

import "fmt"

/*
For a given account, the trace can only have the following pattern

* Non-existing account : READ_ZERO_WS
* Account creation : [INSERT_ST | READ_ZERO_ST ...] INSERT_WS
* Account deleted : [READ_ZERO_ST | READ_NON_ZERO_ST ...] DELETE_WS
* Regular access with write : [ANY_ST ...] UPDATE_WS
* Regular access read-only : [READ_NON_ZERO_ST | READ_ZERO_ST ...] READ_NON_ZERO_WS
* Account redeployed : [READ_ZERO_ST | READ_NON_ZERO_ST ...] DELETE_WS || [INSERT_ST | READ_ZERO_ST ...] INSERT_WS
*/
func inspectPattern(traces []any) (err error) {

	matches := []func(traces []any) (bool, error){
		isMissingAccRead,
		isAccCreation,
		isAccDeletion,
		isAccRead,
		isAccUpdate,
		isAccRedeploy,
	}

	for _, m := range matches {
		ok, err := m(traces)
		if err != nil {
			return err
		}
		// found a match, we can return
		if ok {
			return nil
		}
	}

	// No match was found : return an error with a list of the types
	ts := []string{}
	for i := range traces {
		ts = append(ts, fmt.Sprintf("%T", traces[i]))
	}
	return fmt.Errorf("no match found : %v", ts)
}

// returns true if there is a missing account reading.
// Error, if the trace length is not one.
func isMissingAccRead(traces []any) (ok bool, err error) {

	// Whitelist the pattern : [READ_ZERO_WS]
	if _, ok := traces[0].(ReadZeroTraceWS); ok && len(traces) == 1 {
		return true, nil
	}

	// Look for errors
	for _, trace := range traces {
		if _, ok := trace.(ReadZeroTraceWS); ok {
			return false, fmt.Errorf("found read zero in a trace whose length is larger than 1")
		}
	}

	// Otherwise, it is just a mismatch
	return false, nil
}

// return true if the traces contains only a single INSERT_WS
// at the end error if true and the ST storage are inconsistents
func isAccCreation(traces []any) (ok bool, err error) {
	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].(InsertionTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {
		trace := traces[i]
		// If there is another, then it may be a redeployment
		if isWorldStateTrace(trace) {
			return false, nil
		}

		// Also attempt to cast the trace as a
		switch t := trace.(type) {
		case ReadZeroTraceST, InsertionTraceST:
			// PASS: these are whitelisted operations
		default:
			// Note: at this point we do not know if the error is actual
			// we must return the error outside of the loop. We only keep
			// the first error.
			if err != nil {
				err = fmt.Errorf("invalid trace : found %T in an insertion trace", t)
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	return true, nil
}

// return true if the traces contains only a single INSERT_WS at the end
// error if true and the ST storage are inconsistents
func isAccDeletion(traces []any) (ok bool, err error) {
	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].(DeletionTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {
		trace := traces[i]
		if isWorldStateTrace(trace) {
			return false, fmt.Errorf("invalid deletion preceded by another trace %++v", trace)
		}

		// Also attempt to cast the trace as a
		switch t := trace.(type) {
		case ReadNonZeroTraceST, ReadZeroTraceST:
			// PASS: these are whitelisted operations
		default:
			// Note: at this point we do not know if the error is actual
			// we must return the error outside of the loop. We only keep
			// the first error.
			if err != nil {
				err = fmt.Errorf("invalid trace : found %T in an deletion trace", t)
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	return true, nil
}

// return true if the traces contains only a single UPDATE_WS at the end
// error if true and the ST storage are inconsistents
func isAccUpdate(traces []any) (ok bool, err error) {
	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].(UpdateTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {
		trace := traces[i]
		if isWorldStateTrace(trace) {
			return false, fmt.Errorf("invalid update preceded by another trace %++v", trace)
		}

		// All ST traces are allowed
	}

	return true, nil
}

// return true if the traces contains only a single UPDATE_WS at the end
// error if true and the ST storage are inconsistents
func isAccRead(traces []any) (ok bool, err error) {
	// First check that the last entry is an insertion
	_, ok = traces[0].(ReadNonZeroTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 1; i < len(traces); i++ {
		trace := traces[i]
		if isWorldStateTrace(trace) {
			return false, fmt.Errorf("invalid update preceded by another trace %++v", trace)
		}

		// Test the type of the past traces
		switch t := trace.(type) {
		case ReadNonZeroTraceST, ReadZeroTraceST:
			// PASS: these are whitelisted operations
		default:
			// Note: at this point we do not know if the error is actual
			// we must return the error outside of the loop. We only keep
			// the first error.
			if err != nil {
				err = fmt.Errorf("invalid trace : found %T in an deletion trace", t)
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	// If the length is more than one. Intervert the first and the last entry of the array
	if len(traces) > 1 {
		traces[0], traces[len(traces)-1] = traces[len(traces)-1], traces[0]
	}

	return true, nil
}

func isAccRedeploy(traces []any) (ok bool, err error) {

	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].(InsertionTraceWS)
	if !ok {
		return false, nil
	}

	// Edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return false, nil
	}

	var foundDeletion bool

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {

		// Check if the current trace is a deletion
		_, isDeletion := traces[i].(DeletionTraceWS)

		// Check that there cannot be two deletion
		if isDeletion && foundDeletion {
			return false, fmt.Errorf("found more than two deletion")
		}

		// Else set the foundDeletionFlag to true
		if isDeletion {
			foundDeletion = isDeletion
			continue
		}

		// If the trace is still WS type its invalid
		if isWorldStateTrace(traces[i]) {
			return false, fmt.Errorf("found a %v in a trace ending with an insertion", traces[i])
		}

		// Then the trace must be of type READ
		if !foundDeletion {
			switch t := traces[i].(type) {
			case ReadZeroTraceST, ReadNonZeroTraceST:
				// PASS
			default:
				// Note: at this point we do not know if the error is actual
				// we must return the error outside of the loop. We only keep
				// the first error.
				if err != nil {
					err = fmt.Errorf("invalid trace : found %T in an deletion trace", t)
				}
			}
		}

		if foundDeletion {
			switch t := traces[i].(type) {
			case ReadZeroTraceST, InsertionTraceST:
				// PASS
			default:
				// Note: at this point we do not know if the error is actual
				// we must return the error outside of the loop. We only keep
				// the first error.
				if err != nil {
					err = fmt.Errorf("invalid trace : found %T in an deletion trace", t)
				}
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	return true, nil
}
