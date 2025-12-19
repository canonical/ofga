// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package ofga

var (
	TuplesToOpenFGATupleKeys                        = tuplesToOpenFGATupleKeys
	TupleIsEmpty                                    = (*Tuple).isEmpty
	ValidateTupleForFindMatchingTuples              = validateTupleForFindMatchingTuples
	ValidateTupleForFindUsersByRelation             = validateTupleForFindUsersByRelation
	ValidateTupleForFindAccessibleObjectsByRelation = validateTupleForFindAccessibleObjectsByRelation
	IgnoreMissingOnDelete                           = ignoreMissingOnDelete
	IgnoreDuplicateOnWrite                          = ignoreDuplicateOnWrite
)
