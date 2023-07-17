# surrealtest

Use [ory/dockertest][1] for [SurrealDB][2] to run full integration test with breeze.

[1]: https://github.com/ory/dockertest
[2]: https://hub.docker.com/r/surrealdb/surrealdb

## 🌄 What is `surrealtest`?

`surrealtest` is a package to help set up a SurrealDB local instance on your machine as a part of Go test code. It uses [`ory/dockertest`][2] to start the SurrealDB instance in your Go code, and is configured so that each call to `surrealtest.NewSurrealDB(t)` will create a dedicated instance to allow parallel testing on multiple Docker instances. The function returns a new SurrealDB connection for further testing. `surrealtest.NewSurrealDBRaw(t)` is also available, which returns host+port pair for more complex test scenarios. 

`surrealtest` also provides a helper function `surrealtest.Prepare(t, schema)` to prepare tables and dataset for setting up the table beforehand. The schema can be in arbitrary length, and have complex structure such as defining multiple namespaces and tables, while also populating with some dummy data.

> **Note**: It is a prerequisite that you can start up a Docker container for running SurrealDB with this.

## 🚀 Examples

### Minimal Setup Overview


```go

import (
	"github.com/upsidr/surrealtest"
)

func TestMinimalSetup(t *testing.T) {
	// Create a new SurrealDB instance. Second return value can be called to
	// delete the instance.
	db, clean := surrealtest.NewSurrealDB(t)
	defer clean()

	// You can prepare the database before going into the main test scenario.
	// Any error encountered while setting up is handled with t.Errorf().
	db.Prepare(t, `
	// Comment can be placed based on SurrealQL syntax.

	// Dummy entry for database write testing.
	CREATE x:x SET x = "X";
	CREATE y:y SET y = "Y";
	`)

	// Now the SurrealDB instance is ready, and you can simply interact with
	// the surrealdb.DB struct.
	_ = db

	// Insert user
	data, err := db.Create("user", user)
	if err != nil {
		t.Fatal(err)
	}

	// ...
}
```

You can find more examples in [`helpers_test.go`](helpers_test.go).
