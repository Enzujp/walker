package walker

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name          string
		before, after []Route
		want          Changes
	}{
		{"empty", nil, nil, Changes{Added: []Endpoint{}, Removed: []Endpoint{}}},
		{"method-change", []Route{{Method: "GET", Path: "/"}}, []Route{{Method: "POST", Path: "/"}}, Changes{Added: []Endpoint{{"POST", "/"}}, Removed: []Endpoint{{"GET", "/"}}}},
		{"parameter-rename", []Route{{Method: "GET", Path: "/{id}"}}, []Route{{Method: "GET", Path: "/{userID}"}}, Changes{Added: []Endpoint{{"GET", "/{userID}"}}, Removed: []Endpoint{{"GET", "/{id}"}}}},
		{"metadata-only", []Route{{Method: "get", Path: "/", Summary: "Before"}}, []Route{{Method: "GET", Path: "/", Summary: "After", Body: map[string]string{"x": "y"}}}, Changes{Added: []Endpoint{}, Removed: []Endpoint{}}},
		{"sorted", []Route{{Method: "POST", Path: "/z"}, {Method: "GET", Path: "/a"}}, []Route{{Method: "PUT", Path: "/new"}, {Method: "GET", Path: "/new"}}, Changes{Added: []Endpoint{{"GET", "/new"}, {"PUT", "/new"}}, Removed: []Endpoint{{"GET", "/a"}, {"POST", "/z"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Diff(tt.before, tt.after)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("%+v %v; want %+v", got, err, tt.want)
			}
		})
	}
	for _, pair := range [][2][]Route{
		{[]Route{{Method: "GET", Path: "bad"}}, nil},
		{nil, []Route{{Method: "GET", Path: "/"}, {Method: "GET", Path: "/"}}},
	} {
		if _, err := Diff(pair[0], pair[1]); err == nil {
			t.Fatal("expected validation error")
		}
	}
}
