package surrealtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/gorilla"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/websocket"
)

// NewSurrealDBRaw creates a Docker container with SurrealDB, and returns the
// host:port of the SurrealDB instance. Clean up function is returned as well
// to ensure container gets removed after test is complete.
//
// This function is intended to be used with password "root", and requires
// manual namespace and database handling. While this may be tedious and you may
// be better off using NewSurrealDB instead, this function is useful for testing
// more complex scenarios.
func NewSurrealDBRaw(t testing.TB) (string, func()) {
	t.Helper()

	targetPort := "8000/tcp"

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %v", err)
	}

	runOpt := &dockertest.RunOptions{
		Repository: surrealDBRepo,
		Tag:        surrealDBTag,
		Cmd:        []string{"start", "-p", "root"},

		ExposedPorts: []string{targetPort},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"0/tcp": {{HostIP: "localhost", HostPort: targetPort}},
		},
	}
	resource, err := pool.RunWithOptions(runOpt)
	if err != nil {
		t.Fatalf("Could not start SurrealDB locally: %v", err)
	}

	hostPort := resource.GetHostPort(targetPort)
	t.Logf("Using host:port of '%s'", hostPort)

	if err = pool.Retry(func() error {
		res, err := http.Get("http://" + hostPort + "/status")
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("Could not connect to the Docker instance of SurrealDB: %s", err)
	}

	return hostPort, func() {
		if err = pool.Purge(resource); err != nil {
			t.Fatalf("Could not purge SurrealDB: %s", err)
		}
	}
}

type SurrealDBTest struct {
	*surrealdb.DB
	url       string
	user      string
	pass      string
	namespace string
	database  string
	ws        websocket.WebSocket
}

// NewSurrealDB creates a Docker container with SurrealDB, and returns SurrealDB
// DB connection wrapped in SurrealDBTest struct. Clean up function is returned
// as well to ensure container gets removed after test is complete.
//
// This function assumse the use of "test" namespace and "testdatabase.
func NewSurrealDB(t testing.TB) (*SurrealDBTest, func()) {
	t.Helper()

	hostPort, cleanup := NewSurrealDBRaw(t)

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
		t.Fatalf("failed to establish connection for '%s': %v", url, err)
	}

	login := map[string]string{
		"user": "root",
		"pass": "root",
	}
	if _, err := db.Signin(login); err != nil {
		t.Fatalf("failed to connect using '%+v' for signin: %v", login, err)
	}

	// Select namespace and database
	if _, err := db.Use("test", "test"); err != nil {
		t.Fatalf("failed to use NAMESPACE 'test' and DATABASE 'test': %v", err)
	}

	s := &SurrealDBTest{
		DB:        db,
		url:       url,
		user:      "root",
		pass:      "root",
		namespace: "test",
		database:  "test",
		ws:        ws,
	}
	return s, func() {
		db.Close()
		cleanup()
	}
}

// Prepare allows creating tables with the target database. The schema input
// can contain any number of SurrealQL expressions, including data creation,
// changing namespace and database, etc. If you are to provide multiple
// statements, make sure to separate them with a semicolon.
func (s *SurrealDBTest) Prepare(t testing.TB, schema string) {
	t.Helper()

	// Use SurrealDB's raw query support. This expects no complex stringg
	// manipulation, and thus the second param is set to nil.
	x, err := s.Query(schema, nil)
	if err != nil {
		t.Fatalf("failed to define tables: %v", err)
	}
	if err := checkQueryResponse(x); err != nil {
		t.Errorf("failed to define tables:\n  %v", err)
	}
}

type queryResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Detail string `json:"detail"`
}

// checkQueryResponse checks the response bytes which is expected to be a json
// input, and returns an error if any error is found in that response. If there
// are multiple errors found, all of the error details are described as a part
// of the error message.
func checkQueryResponse(data interface{}) error {
	var err error
	var ok bool
	d := data
	if isSlice(data) {
		d, ok = data.([]interface{})
		if !ok {
			return errors.New("failed to deserialise response to slice")
		}
	}
	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("failed to deserialise response '%+v' to slice", d)
	}

	var responses []queryResponse
	err = json.Unmarshal(jsonBytes, &responses)
	if err != nil {
		return fmt.Errorf("failed unmarshaling jsonBytes '%+v': %w", jsonBytes, err)
	}

	errs := []string{}
	for _, r := range responses {
		if r.Status != "OK" {
			errs = append(errs, r.Detail)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to execute query:\n    %s", strings.Join(errs, "\n    "))
	}

	return nil
}

func isSlice(possibleSlice interface{}) bool {
	val := reflect.ValueOf(possibleSlice)
	return val.Kind() == reflect.Slice
}

func toSliceOfAny[T any](s []T) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
