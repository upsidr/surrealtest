package surrealtest

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Temporarily placed here, PR is created for upstream surrealdb.
// Ref: https://github.com/surrealdb/surrealdb.go/pull/79
func SmartUnmarshalAll[I any](input interface{}) ([]I, error) {
	return handleInterfaces[I](input)
}

func handleInterfaces[I any](input interface{}) ([]I, error) {
	var result []I
	var errs []error
	inputs, ok := input.([]interface{})
	if !ok {
		fmt.Printf("  data\n    %+v\n", input)
		return handleInput[I](input)
	}

	for _, i := range inputs {
		data, err := handleInterfaces[I](i)
		errs = append(errs, err)
		result = append(result, data...)
	}
	if len(errs) > 0 {
		return result, errors.Join(errs...)
	}
	return result, nil
}

var errNotRawQuery = fmt.Errorf("not a RawQuery")

// handleInput takes in an interface input and unmarshals it into the given
// type, regardless of the input wrapped in RawQuery or not. The input is
// expected to be a single entry, rather than a slice.
//
// In case of the input being a RawQuery, though, it could have a slice inside,
// and thus would recurse to check for interface input.
func handleInput[I any](input interface{}) ([]I, error) {
	unwrapped, err := handleAsRawQuery(input)
	if err != nil {
		// If error has to do with the input status, return the error.
		if !errors.Is(err, errNotRawQuery) {
			return nil, err
		}

		// If error is unmarshaling failure to RawQuery format, try to get the
		// single data as is.
		data, err := handleAsData[I](input)
		if err != nil {
			return nil, err
		}
		return []I{data}, nil
	}
	return handleInterfaces[I](unwrapped)
}

// handleAsData takes in an interface input and unmarshals it into the given
// type. The input is expected to be a single entry, rather than a slice.
func handleAsData[I any](input interface{}) (I, error) {
	var i I
	x, err := json.Marshal(input)
	if err != nil {
		return i, fmt.Errorf("failed to marshal to bytes: %w", err)
	}
	err = json.Unmarshal(x, &i)
	if err != nil {
		return i, fmt.Errorf("failed to unmarshal to RawQuery: %w", err)
	}
	return i, nil
}

// handleAsRawQuery takes in an interface input and unmarshals it into the
// RawQuery format. The input is expected to be a single entry, rather than a
// slice. It would return `errNotRawQuery` if the input is not in the RawQuery,
// so that the rest of the process could try unmarshalling against a separate
// type.
func handleAsRawQuery(input interface{}) (interface{}, error) {
	x, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to bytes: %w", err)
	}

	var raw RawQuery[any]
	err = json.Unmarshal(x, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to RawQuery: %w", err)
	}

	if raw.Status == "" {
		return nil, fmt.Errorf("%w", errNotRawQuery)
	}

	if raw.Status != "OK" {
		return nil, fmt.Errorf("%s: %s", raw.Status, raw.Result)
	}

	return raw.Result, nil
}

// Used for RawQuery Unmarshaling
type RawQuery[I any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result I      `json:"result"`
	Detail string `json:"detail"`
}
