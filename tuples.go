package ofga

import (
	openfga "github.com/openfga/go-sdk"
)

// Kind represents the type of the entity in OpenFGA.
type Kind string

// String implements the Stringer interface.
func (k Kind) String() string {
	return string(k)
}

// Relation represents the type of relation between entities in OpenFGA.
type Relation string

// String implements the Stringer interface.
func (r Relation) String() string {
	return string(r)
}

// Entity represents an entity/entity-set in OpenFGA.
// Example: `user:<user-id>`, `org:<org-id>#member`
type Entity struct {
	Kind     Kind
	ID       string
	Relation Relation
}

// String returns a string representation of the entity/entity-set.
func (e *Entity) String() string {
	if e.Relation == "" {
		return string(e.Kind) + ":" + e.ID
	}
	return e.Kind.String() + ":" + e.ID + "#" + e.Relation.String()
}

// Tuple represents a relation between an object and a target. Note that OpenFGA
// represents a Tuple as (User, Relation, Object). However, the `User` field is
// not restricted to just being users, it could also refer to objects when we
// need to create object to object relationships. Hence, we chose to use
// (Object, Relation, Target), as it results in more consistent naming.
type Tuple struct {
	Object   *Entity
	Relation Relation
	Target   *Entity
}

// toOpenFGATuple converts our Tuple struct into an OpenFGA TupleKey
func (t Tuple) toOpenFGATuple() openfga.TupleKey {
	k := openfga.NewTupleKey()
	// in some cases specifying the object is not required
	if t.Object != nil {
		k.SetUser(t.Object.String())
	}
	// in some cases specifying the relation is not required
	if t.Relation != "" {
		k.SetRelation(string(t.Relation))
	}
	k.SetObject(t.Target.String())
	return *k
}
