// Copyright 2023 Canonical Ltd.
// Licensed under the LGPL license, see LICENSE file for details.

package ofga_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	openfga "github.com/openfga/go-sdk"

	"github.com/canonical/ofga"
)

const (
	relationEditor ofga.Relation = "editor"
	relationViewer ofga.Relation = "viewer"
)

var (
	entityTestUser     = ofga.Entity{Kind: "user", ID: "123"}
	entityTestUser2    = ofga.Entity{Kind: "user2", ID: "456"}
	entityTestContract = ofga.Entity{Kind: "contract", ID: "789"}
	authModelJson      = []byte(`{
	  "type_definitions": [
		{
		  "type": "user",
		  "relations": {},
		  "metadata": null
		},
		{
		  "type": "document",
		  "relations": {
			"viewer": {
			  "union": {
				"child": [
				  {
					"this": {}
				  },
				  {
					"computedUserset": {
					  "object": "",
					  "relation": "writer"
					}
				  }
				]
			  }
			},
			"writer": {
			  "this": {}
			}
		  },
		  "metadata": {
			"relations": {
			  "viewer": {
				"directly_related_user_types": [
				  {
					"type": "user"
				  }
				]
			  },
			  "writer": {
				"directly_related_user_types": [
				  {
					"type": "user"
				  }
				]
			  }
			}
		  }
		}
	  ],
	  "schema_version": "1.1"
	}`)
	authModel = openfga.AuthorizationModel{
		SchemaVersion: "1.1",
		TypeDefinitions: &[]openfga.TypeDefinition{
			{Type: "user", Relations: &map[string]openfga.Userset{}},
			{
				Type: "document",
				Relations: &map[string]openfga.Userset{
					"writer": {
						This: &map[string]interface{}{},
					},
					"viewer": {Union: &openfga.Usersets{
						Child: &[]openfga.Userset{
							{This: &map[string]interface{}{}},
							{ComputedUserset: &openfga.ObjectRelation{
								Object:   openfga.PtrString(""),
								Relation: openfga.PtrString("writer"),
							}},
						},
					}},
				},
				Metadata: &openfga.Metadata{
					Relations: &map[string]openfga.RelationMetadata{
						"writer": {
							DirectlyRelatedUserTypes: &[]openfga.RelationReference{
								{Type: "user"},
							},
						},
						"viewer": {
							DirectlyRelatedUserTypes: &[]openfga.RelationReference{
								{Type: "user"},
							},
						},
					},
				},
			},
		},
	}
)

func TestToOpenFGATuple(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about                   string
		tuple                   ofga.Tuple
		expectedOpenFGATupleKey openfga.TupleKey
	}{{
		about: "tuple with object, relation and target is converted successfully",
		tuple: ofga.Tuple{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			User:     openfga.PtrString(entityTestUser.String()),
			Relation: openfga.PtrString(relationEditor.String()),
			Object:   openfga.PtrString(entityTestContract.String()),
		},
	}, {
		about: "tuple with relation and target is converted successfully",
		tuple: ofga.Tuple{
			Relation: relationEditor,
			Target:   &entityTestContract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			Relation: openfga.PtrString(relationEditor.String()),
			Object:   openfga.PtrString(entityTestContract.String()),
		},
	}, {
		about: "tuple with object and target is converted successfully",
		tuple: ofga.Tuple{
			Object: &entityTestUser,
			Target: &entityTestContract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			User:   openfga.PtrString(entityTestUser.String()),
			Object: openfga.PtrString(entityTestContract.String()),
		},
	}, {
		about: "tuple with only target is converted successfully",
		tuple: ofga.Tuple{
			Target: &entityTestContract,
		},
		expectedOpenFGATupleKey: openfga.TupleKey{
			Object: openfga.PtrString(entityTestContract.String()),
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

func TestTuplesToOpenFGATupleKeys(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		about                    string
		tuples                   []ofga.Tuple
		expectedOpenFGATupleKeys []openfga.TupleKey
	}{{
		about:                    "empty slice of tuples returns empty slice of tuple keys",
		tuples:                   []ofga.Tuple{},
		expectedOpenFGATupleKeys: []openfga.TupleKey{},
	}, {
		about: "tuples converted successfully",
		tuples: []ofga.Tuple{{
			Object:   &entityTestUser,
			Relation: relationEditor,
			Target:   &entityTestContract,
		}, {
			Relation: relationEditor,
			Target:   &entityTestContract,
		}, {
			Target: &entityTestContract,
		}},
		expectedOpenFGATupleKeys: []openfga.TupleKey{{
			User:     openfga.PtrString(entityTestUser.String()),
			Relation: openfga.PtrString(relationEditor.String()),
			Object:   openfga.PtrString(entityTestContract.String()),
		}, {
			Relation: openfga.PtrString(relationEditor.String()),
			Object:   openfga.PtrString(entityTestContract.String()),
		}, {
			Object: openfga.PtrString(entityTestContract.String()),
		}},
	}}

	for _, test := range tests {
		test := test
		c.Run(test.about, func(c *qt.C) {
			c.Parallel()

			tupleKeys := ofga.TuplesToOpenFGATupleKeys(test.tuples)
			c.Assert(tupleKeys, qt.DeepEquals, test.expectedOpenFGATupleKeys)
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
	}, {
		about:        "entity with special characters in ID is parsed correctly",
		entityString: "user:some.user-name+suffix@some.domain-name+suffix",
		expectedEntity: ofga.Entity{
			Kind: "user",
			ID:   "some.user-name+suffix@some.domain-name+suffix",
		},
	}, {
		about:        "entity with special characters in ID and a relation is parsed correctly",
		entityString: "user:some.user-name+suffix@some.domain-name+suffix#member",
		expectedEntity: ofga.Entity{
			Kind:     "user",
			ID:       "some.user-name+suffix@some.domain-name+suffix",
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
