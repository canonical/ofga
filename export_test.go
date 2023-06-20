// Copyright 2023 Canonical Ltd.

package ofga

var (
	ToOpenFGATuple                                  = (*Tuple).toOpenFGATuple
	TupleIsEmpty                                    = (*Tuple).isEmpty
	FromOpenFGATupleKey                             = fromOpenFGATupleKey
	ValidateTupleForFindMatchingTuples              = validateTupleForFindMatchingTuples
	ValidateTupleForFindUsersByRelation             = validateTupleForFindUsersByRelation
	TraverseTree                                    = (*Client).traverseTree
	Expand                                          = (*Client).expand
	ExpandComputed                                  = (*Client).expandComputed
	ValidateTupleForFindAccessibleObjectsByRelation = validateTupleForFindAccessibleObjectsByRelation
)
