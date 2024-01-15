// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package ofga

import (
	"fmt"
	"regexp"
	"time"

	openfga "github.com/openfga/go-sdk"
)

// entityRegex is used to validate that a string represents an Entity/EntitySet
// and helps to convert from a string representation into an Entity struct.
var entityRegex = regexp.MustCompile(`([A-Za-z0-9_][A-Za-z0-9_-]*):([A-Za-z0-9_][A-Za-z0-9_@.+-]*|[*])(#([A-Za-z0-9_][A-Za-z0-9_-]*))?$`)

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

// IsPublicAccess returns true when the entity ID is the * wildcard, representing any entity.
func (e *Entity) IsPublicAccess() bool {
	return e.ID == "*"
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
//     eg. organization:canonical#member
func ParseEntity(s string) (Entity, error) {
	match := entityRegex.FindStringSubmatch(s)
	if match == nil {
		return Entity{}, fmt.Errorf("invalid entity representation: %s", s)
	}

	// Extract and return the relevant information from the sub-matches.
	return Entity{
		Kind:     Kind(match[1]),
		ID:       match[2],
		Relation: Relation(match[4]),
	}, nil
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

// ToOpenFGATupleKey converts our Tuple struct into an OpenFGA TupleKey.
func (t Tuple) ToOpenFGATupleKey() openfga.TupleKey {
	k := openfga.NewTupleKeyWithDefaults()
	// In some cases, specifying the object is not required.
	if t.Object != nil {
		k.SetUser(t.Object.String())
	}
	// In some cases, specifying the relation is not required.
	if t.Relation != "" {
		k.SetRelation(t.Relation.String())
	}
	k.SetObject(t.Target.String())
	return *k
}

// ToOpenFGACheckRequestTupleKey converts our Tuple struct into an
// OpenFGA CheckRequestTupleKey.
func (t Tuple) ToOpenFGACheckRequestTupleKey() openfga.CheckRequestTupleKey {
	tk := t.ToOpenFGATupleKey()
	return *openfga.NewCheckRequestTupleKey(tk.User, tk.Relation, tk.Object)
}

// ToOpenFGAExpandRequestTupleKey converts our Tuple struct into an
// OpenFGA ExpandRequestTupleKey.
func (t Tuple) ToOpenFGAExpandRequestTupleKey() openfga.ExpandRequestTupleKey {
	tk := t.ToOpenFGATupleKey()
	return *openfga.NewExpandRequestTupleKey(tk.Relation, tk.Object)
}

// ToOpenFGAReadRequestTupleKey converts our Tuple struct into an
// OpenFGA ReadRequestTupleKey.
func (t Tuple) ToOpenFGAReadRequestTupleKey() openfga.ReadRequestTupleKey {
	k := openfga.NewReadRequestTupleKeyWithDefaults()
	// In some cases, specifying the object is not required.
	if t.Object != nil {
		k.SetUser(t.Object.String())
	}
	// In some cases, specifying the relation is not required.
	if t.Relation != "" {
		k.SetRelation(t.Relation.String())
	}
	k.SetObject(t.Target.String())
	return *k
}

// ToOpenFGATupleKeyWithoutCondition converts our Tuple struct into an
// OpenFGA TupleKeyWithoutCondition.
func (t Tuple) ToOpenFGATupleKeyWithoutCondition() openfga.TupleKeyWithoutCondition {
	tk := t.ToOpenFGATupleKey()
	return *openfga.NewTupleKeyWithoutCondition(tk.User, tk.Relation, tk.Object)
}

// FromOpenFGATupleKey converts an openfga.TupleKey struct into a Tuple.
func FromOpenFGATupleKey(key openfga.TupleKey) (Tuple, error) {
	var user, object Entity
	var err error
	if key.User != "" {
		user, err = ParseEntity(key.GetUser())
		if err != nil {
			return Tuple{}, err
		}
	}
	if key.Object != "" {
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

// tuplesToOpenFGATupleKeys converts a slice of tuples into OpenFGA TupleKeys.
func tuplesToOpenFGATupleKeys(tuples []Tuple) []openfga.TupleKey {
	keys := make([]openfga.TupleKey, len(tuples))
	for i, tuple := range tuples {
		keys[i] = tuple.ToOpenFGATupleKey()
	}
	return keys
}

// tuplesToOpenFGATupleKeysWithoutCondition converts a slice of tuples into
// a slice of OpenFGA TupleKeyWithoutCondition.
func tuplesToOpenFGATupleKeysWithoutCondition(tuples []Tuple) []openfga.TupleKeyWithoutCondition {
	keys := make([]openfga.TupleKeyWithoutCondition, len(tuples))
	for i, tuple := range tuples {
		keys[i] = tuple.ToOpenFGATupleKeyWithoutCondition()
	}
	return keys
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
