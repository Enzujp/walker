package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/enzujp/walker/pkg/walker"
)

func decodeDocument(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected exactly one JSON document")
		}
		return fmt.Errorf("trailing JSON data: %w", err)
	}
	return nil
}

func readManifest(path string, stdin io.Reader) ([]walker.Route, error) {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open manifest %q: %w", path, err)
		}
		defer file.Close()
		reader = file
	}
	var routes []walker.Route
	if err := decodeDocument(reader, &routes); err != nil {
		return nil, fmt.Errorf("decode manifest %q: %w", path, err)
	}
	if routes == nil {
		return nil, fmt.Errorf("manifest %q must be a JSON array, not null", path)
	}
	routes, err := walker.Normalize(routes)
	if err != nil {
		return nil, fmt.Errorf("manifest %q: %w", path, err)
	}
	return routes, nil
}

func readOptions(path string) (walker.Options, error) {
	file, err := os.Open(path)
	if err != nil {
		return walker.Options{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()
	var options *walker.Options
	if err := decodeDocument(file, &options); err != nil {
		return walker.Options{}, fmt.Errorf("decode config: %w", err)
	}
	if options == nil {
		return walker.Options{}, fmt.Errorf("config must be a JSON object, not null")
	}
	return *options, nil
}
