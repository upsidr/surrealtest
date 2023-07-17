package surrealtest_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/gorilla"
	"github.com/surrealdb/surrealdb.go/pkg/logger"

	"github.com/upsidr/surrealtest"
)

func TestSimple(t *testing.T) {
	// Because surrealtest is designed to create a separate SurrealDB instance
	// for each test, it is safe to run all the tests in parallel.
	t.Parallel()

	db, clean := surrealtest.NewSurrealDB(t)
	// By not calling clean, the database can be left running after the test.
	defer clean()
	// _ = clean

	db.Prepare(t, `
    // Comment can be placed based on SurrealQL syntax.

	// Dummy entry for database write testing.
	CREATE x:x SET x = "X";
	CREATE y:y SET y = "Y";

	// Define user table.
	DEFINE TABLE user;
	DEFINE FIELD name ON TABLE user ASSERT $value != none;

	// Create user entries, and because this one does not specify ID,
	// SurrealDB will generate a random ID for this entry.
	CREATE user SET name = "John";
	CREATE user SET name = "Johnny";
    `)

	type User struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		ID       string `json:"id,omitempty"`
		Name     string `json:"name,omitempty"`
	}
	// Create user struct
	user := User{
		ID:       "xxx", // If this conflicts, Create will fail.
		Username: "John",
		Password: "X",
		Name:     "John",
	}

	// Insert user
	data, err := db.Create("user", user)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal data
	//
	// There are several ways for this.

	// A. Use SmartUnmarshalAll
	x, err := surrealtest.SmartUnmarshalAll[User](data)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("created user, data returned by SmartUnmarshalAll:\n    %+v", x)

	// B. Use SmartUnmarshal
	x, err = surrealdb.SmartUnmarshal[[]User](data, err)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("created user, data returned by SmartUnmarshal:\n    %v", x)

	// C. Use simple Unmarshal
	x = make([]User, 1)
	err = surrealdb.Unmarshal(data, &x)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("created user, data returned by Unmarshal:\n    %+v", x)

	// Get user by ID
	data, err = db.Select(x[0].ID)
	x, err = surrealtest.SmartUnmarshalAll[User](data)

	t.Logf("select user, data returned by SmartUnmarshalAll:\n    %v", x)

	// Get all users
	data, err = db.Select("user")
	x, err = surrealtest.SmartUnmarshalAll[User](data)

	t.Logf("select user all, data returned by SmartUnmarshalAll:\n    %v", x)
}

func TestSimpleRaw(t *testing.T) {
	// Because surrealtest is designed to create a separate SurrealDB instance
	// for each test, it is safe to run all the tests in parallel.
	t.Parallel()

	hostPort, clean := surrealtest.NewSurrealDBRaw(t)
	// By not calling clean, the database can be left running after the test.
	defer clean()
	// _ = clean

	url := "ws://" + hostPort + "/rpc"
	buff := bytes.NewBuffer([]byte{})
	logData, err := logger.New().FromBuffer(buff).Make()
	if err != nil {
		t.Fatalf("failed to set up logger: %v", err)
	}
	ws, err := gorilla.Create().SetTimeOut(5 * time.Second).SetCompression(true).Logger(logData).Connect(url)
	if err != nil {
		t.Fatalf("failed to create websocket connection: %v", err)
	}
	db, err := surrealdb.New(url, ws)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Sign in
	if _, err := db.Signin(map[string]string{
		"user": "root",
		"pass": "root",
	}); err != nil {
		t.Fatal(err)
	}

	// Select namespace and database with use
	if _, err := db.Use("test", "test"); err != nil {
		t.Fatal(err)
	}

	type User struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		ID       string `json:"id,omitempty"`
	}
	// Create user struct
	user := User{
		Username: "John",
		Password: "X",
		ID:       "xxx",
	}

	// Insert user
	data, err := db.Create("user", user)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal data
	createdUser := make([]User, 1)
	err = surrealdb.Unmarshal(data, &createdUser)
	if err != nil {
		t.Fatal(err)
	}

	// Get user by ID
	data, err = db.Select(createdUser[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("data: %+v", data)
}

func isSlice(possibleSlice interface{}) bool {
	val := reflect.ValueOf(possibleSlice)
	return val.Kind() == reflect.Slice
}
