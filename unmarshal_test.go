package surrealtest_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/upsidr/surrealtest"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	type User struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		ID       string `json:"id,omitempty"`
		Name     string `json:"name,omitempty"`
	}

	prep := func(t testing.TB, db *surrealtest.SurrealDBTest) {
		db.Prepare(t, `
	// Comment can be placed based on SurrealQL syntax.

	// Dummy entry for database write testing
	CREATE x:x SET x = "X";
	CREATE y:y SET y = "Y";

	// Define user table
	DEFINE TABLE user;
	DEFINE FIELD name ON TABLE user ASSERT $value != none;

	// Create a user entry, and because this one does not specify ID,
	// SurrealDB will generate a random ID for this entry.
	CREATE user:john   SET name = "John";
	CREATE user:johnny SET name = "Johnny";
	`)
	}

	cases := map[string]struct {
		interaction  func(db *surrealtest.SurrealDBTest) (interface{}, error)
		wantData     interface{}
		wantErrorMsg string
	}{
		"Create: success": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Create("user", User{
					ID:       "xxx", // If this conflicts, Create will fail.
					Username: "John",
					Password: "X",
					Name:     "John",
				})
			},
			wantData: []User{
				{
					ID:       "user:xxx",
					Username: "John",
					Password: "X",
					Name:     "John",
				},
			},
		},
		"Select: success": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Select("user")
			},
			wantData: []User{
				{
					ID:   "user:john",
					Name: "John",
				},
				{
					ID:   "user:johnny",
					Name: "Johnny",
				},
			},
		},
		"Select: malformed": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Select("* from user") // Not valid, but doesn't return error.
			},
			wantData: []User(nil), // No data is returned.
		},
		"Query: success": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Query(
					`
					 CREATE user:j  SET name = "J";
					 CREATE user:jo SET name = "Jo";
					`, nil)
			},
			wantData: []User{
				{
					ID:   "user:j",
					Name: "J",
				},
				{
					ID:   "user:jo",
					Name: "Jo",
				},
			},
		},
		"Query: fail with duplicated create": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Query(
					`
					 CREATE user:j SET name = "J";
					 CREATE user:j SET name = "J"; // conflict
					 CREATE user SET name = "John";
					`, nil)
			},
			wantErrorMsg: "Database record `user:j` already exists",
		},
		"Transaction: success": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Query(
					`
					 BEGIN TRANSACTION;
					 CREATE user:j  SET name = "J";
					 CREATE user:jo SET name = "Jo";
					 COMMIT TRANSACTION;
					`, nil)
			},
			wantData: []User{
				{
					ID:   "user:j",
					Name: "J",
				},
				{
					ID:   "user:jo",
					Name: "Jo",
				},
			},
		},
		"Transaction: fail with duplicated create": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Query(
					`
					 BEGIN TRANSACTION;
					 CREATE user:j SET name = "J";
					 CREATE user:j SET name = "J"; // conflict
					 CREATE user SET name = "John";
					 COMMIT TRANSACTION;
					`, nil)
			},
			wantErrorMsg: "Database record `user:j` already exists",
		},
		"Transaction: canceled": {
			interaction: func(db *surrealtest.SurrealDBTest) (interface{}, error) {
				return db.Query(
					`
					 BEGIN TRANSACTION;
					 CREATE user:j SET name = "J";
					 CREATE user SET name = "John";
					 CANCEL TRANSACTION;
					`, nil)
			},
			wantErrorMsg: "cancelled transaction",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			db, clean := surrealtest.NewSurrealDB(t)
			defer clean()
			prep(t, db)

			data, err := tc.interaction(db)
			if err != nil {
				_ = strings.Join([]string{}, "")
				// if strings.Contains(err.Error(), tc.wantErrorMsg) {
				// 	t.Errorf("unexpected error message:\n    want: %v\n    got:  %v",
				// 		tc.wantErrorMsg, err)
				// }

				// return
				t.Fatalf("failed with err for first interaction: %v", err)
			}

			x, err := surrealtest.SmartUnmarshalAll[User](data)
			if err != nil {
				t.Logf("REFERENCE: %+v", x)

				if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("unexpected error message:\n    want: %v\n    got:  %v",
						tc.wantErrorMsg, err)
				}
				return
			}
			if diff := cmp.Diff(tc.wantData, x); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}

}
