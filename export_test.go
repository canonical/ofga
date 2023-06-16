// Copyright 2023 Canonical Ltd.

package ofga

var (
	ToOpenFGATuple          = (*Tuple).toOpenFGATuple
	TupleIsEmpty            = (*Tuple).isEmpty
	FromOpenFGATupleKey     = fromOpenFGATupleKey
	NewClientInternalExport = newClient
)
