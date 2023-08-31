// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package ofga

var (
	ToOpenFGATuple                                  = (*Tuple).toOpenFGATupleKey
	TuplesToOpenFGATupleKeys                        = tuplesToOpenFGATupleKeys
	TupleIsEmpty                                    = (*Tuple).isEmpty
	FromOpenFGATupleKey                             = fromOpenFGATupleKey
	ValidateTupleForFindMatchingTuples              = validateTupleForFindMatchingTuples
	ValidateTupleForFindUsersByRelation             = validateTupleForFindUsersByRelation
	FindUsersByRelationInternal                     = (*Client).findUsersByRelation
	TraverseTree                                    = (*Client).traverseTree
	Expand                                          = (*Client).expand
	ExpandComputed                                  = (*Client).expandComputed
	ValidateTupleForFindAccessibleObjectsByRelation = validateTupleForFindAccessibleObjectsByRelation
)
