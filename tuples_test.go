// Copyright 2023 Canonical Ltd.

package ofga_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	openfga "github.com/openfga/go-sdk"

	"github.com/canonical/ofga"
)

const Editor ofga.Relation = "editor"

func TestToOpenFGATuple(t *testing.T) {
	c := qt.New(t)
	user := ofga.Entity{
		Kind: "user",
		ID:   "123",
	}
	contract := ofga.Entity{
		Kind: "contract",
		ID:   "789",
	}

	tests := []struct {
		about                   string
		tuple                   ofga.Tuple
		expectedOpenFGATupleKey openfga.TupleKey
	}{{
		about: "tuple with object, relation and target is converted successfully",
		tuple: ofga.Tuple{
			Object:   &user,
			Relation: Editor,
			Target:   &contract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			User:     openfga.PtrString(user.String()),
			Relation: openfga.PtrString(Editor.String()),
			Object:   openfga.PtrString(contract.String()),
		},
	}, {
		about: "tuple with relation and target is converted successfully",
		tuple: ofga.Tuple{
			Relation: Editor,
			Target:   &contract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			Relation: openfga.PtrString(Editor.String()),
			Object:   openfga.PtrString(contract.String()),
		},
	}, {
		about: "tuple with object and target is converted successfully",
		tuple: ofga.Tuple{
			Object: &user,
			Target: &contract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			User:   openfga.PtrString(user.String()),
			Object: openfga.PtrString(contract.String()),
		},
	}, {
		about: "tuple with only target is converted successfully",
		tuple: ofga.Tuple{
			Target: &contract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			Object: openfga.PtrString(contract.String()),
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			tupleKey := ofga.ToOpenFGATuple(&test.tuple)
			c.Assert(tupleKey, qt.DeepEquals, test.expectedOpenFGATupleKey)
		})
	}
}

func TestEntity_String(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about          string
		entity         ofga.Entity
		expectedString string
	}{{
		about: "entity without a relation is correctly represented",
		entity: ofga.Entity{
			Kind: "user",
			ID:   "123",
		},
		expectedString: "user:123",
	}, {
		about: "entity with a relation is correctly represented",
		entity: ofga.Entity{
			Kind:     "organization",
			ID:       "ABC",
			Relation: "member",
		},
		expectedString: "organization:ABC#member",
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			s := test.entity.String()
			c.Assert(s, qt.DeepEquals, test.expectedString)
		})
	}
}

func TestParseEntity(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about          string
		entityString   string
		expectedEntity ofga.Entity
		expectedErr    string
	}{{
		about:        "malformed entity representation raises an error",
		entityString: "organization#member",
		expectedErr:  "invalid entity representation.*",
	}, {
		about:        "entity without a relation is parsed correctly",
		entityString: "organization:canonical",
		expectedEntity: ofga.Entity{
			Kind: "organization",
			ID:   "canonical",
		},
	}, {
		about:        "entity with a relation is parsed correctly",
		entityString: "organization:canonical#member",
		expectedEntity: ofga.Entity{
			Kind:     "organization",
			ID:       "canonical",
			Relation: "member",
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			entity, err := ofga.ParseEntity(test.entityString)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(entity, qt.DeepEquals, test.expectedEntity)
			}
		})
	}
}

func TestFromOpenFGATupleKey(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about         string
		tupleKey      openfga.TupleKey
		expectedTuple ofga.Tuple
		expectedErr   string
	}{{
		about: "tuple with malformed user entity raises error",
		tupleKey: openfga.TupleKey{
			User:     openfga.PtrString("user#XYZ"),
			Relation: openfga.PtrString("member"),
			Object:   openfga.PtrString("organization:canonical"),
		},
		expectedErr: "invalid entity representation.*",
	}, {
		about: "tuple with malformed object entity raises error",
		tupleKey: openfga.TupleKey{
			User:     openfga.PtrString("user:XYZ"),
			Relation: openfga.PtrString("member"),
			Object:   openfga.PtrString("organization"),
		},
		expectedErr: "invalid entity representation.*",
	}, {
		about: "tuple with all valid fields is converted successfully",
		tupleKey: openfga.TupleKey{
			User:     openfga.PtrString("user:XYZ"),
			Relation: openfga.PtrString("member"),
			Object:   openfga.PtrString("organization:canonical"),
		},
		expectedTuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "member",
			Target:   &ofga.Entity{Kind: "organization", ID: "canonical"},
		},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			tuple, err := ofga.FromOpenFGATupleKey(test.tupleKey)

			if test.expectedErr != "" {
				c.Assert(err, qt.ErrorMatches, test.expectedErr)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(tuple, qt.DeepEquals, test.expectedTuple)
			}
		})
	}
}

func TestTuple_IsEmpty(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about           string
		tuple           ofga.Tuple
		expectedIsEmpty bool
	}{{
		about: "tuple with an Object is not empty",
		tuple: ofga.Tuple{
			Object:   &ofga.Entity{Kind: "user", ID: "XYZ"},
			Relation: "",
			Target:   nil,
		},
		expectedIsEmpty: false,
	}, {
		about: "tuple with a Relation is not empty",
		tuple: ofga.Tuple{
			Object:   nil,
			Relation: "member",
			Target:   nil,
		},
		expectedIsEmpty: false,
	}, {
		about: "tuple with a Target is not empty",
		tuple: ofga.Tuple{
			Object:   nil,
			Relation: "",
			Target:   &ofga.Entity{Kind: "organization", ID: "canonical"},
		},
		expectedIsEmpty: false,
	}, {
		about: "tuple without an Object, Relation or Target is empty",
		tuple: ofga.Tuple{
			Object:   nil,
			Relation: "",
			Target:   nil,
		},
		expectedIsEmpty: true,
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			isEmpty := ofga.TupleIsEmpty(&test.tuple)

			c.Assert(isEmpty, qt.Equals, test.expectedIsEmpty)
		})
	}
}
