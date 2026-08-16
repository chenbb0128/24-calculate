package httpapi

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIYamlIsParseable(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "openapi.yaml")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(payload, &document); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	if len(document.Content) == 0 {
		t.Fatal("OpenAPI document is empty")
	}
}
