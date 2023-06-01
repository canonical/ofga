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
