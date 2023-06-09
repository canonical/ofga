// Copyright 2023 Canonical Ltd.

package ofga

import (
	"fmt"
	"regexp"
	"time"

	openfga "github.com/openfga/go-sdk"
)

var EntityRegex = regexp.MustCompile(`([A-za-z0-9_][A-za-z0-9_-]*):([A-za-z0-9_][A-za-z0-9_-]*)(#([A-za-z0-9_][A-za-z0-9_-]*))?`)

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
		return e.Kind.String() + ":" + e.ID
	}
	return e.Kind.String() + ":" + e.ID + "#" + e.Relation.String()
}

// ParseEntity will parse a string representation into an Entity. It expects to
// find entities of the form:
//   - <entityType>:<ID>
//     eg. organization:canonical
//   - <entityType>:<ID>#<relationship-set>
//     eg. organization:canonical#members
func ParseEntity(s string) (Entity, error) {
	match := EntityRegex.FindStringSubmatch(s)
	switch len(match) {
	case 3:
		return Entity{
			Kind: Kind(match[1]),
			ID:   match[2],
		}, nil
	case 5:
		return Entity{
			Kind:     Kind(match[1]),
			ID:       match[2],
			Relation: Relation(match[4]),
		}, nil
	default:
		return Entity{}, fmt.Errorf("invalid entity representation: %s", s)
	}
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

// toOpenFGATuple converts our Tuple struct into an OpenFGA TupleKey.
func (t Tuple) toOpenFGATuple() openfga.TupleKey {
	k := openfga.NewTupleKey()
	// in some cases specifying the object is not required
	if t.Object != nil {
		k.SetUser(t.Object.String())
	}
	// in some cases specifying the relation is not required
	if t.Relation != "" {
		k.SetRelation(t.Relation.String())
	}
	k.SetObject(t.Target.String())
	return *k
}

// fromOpenFGATupleKey converts an openfga.TupleKey struct into a Tuple.
func fromOpenFGATupleKey(key openfga.TupleKey) (Tuple, error) {
	var user, object Entity
	var err error
	if key.HasUser() {
		user, err = ParseEntity(key.GetUser())
		if err != nil {
			return Tuple{}, err
		}
	}
	if key.HasObject() {
		object, err = ParseEntity(key.GetObject())
		if err != nil {
			return Tuple{}, err
		}
	}

	return Tuple{
		Object:   &user,
		Relation: Relation(key.GetRelation()),
		Target:   &object,
	}, nil
}

// isEmpty is a helper method to check whether a tuple is set to a non-empty
// value or not.
func (t Tuple) isEmpty() bool {
	if t.Object == nil && t.Relation == "" && t.Target == nil {
		return true
	}
	return false
}

// TimestampedTuple is a tuple accompanied by a timestamp that represents
// the timestamp at which the tuple was created.
type TimestampedTuple struct {
	Tuple     Tuple
	Timestamp time.Time
}
