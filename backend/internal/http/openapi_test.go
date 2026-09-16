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

func TestOpenAPIDocumentsAdminAPIContract(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "openapi.yaml")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(payload, &document); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	paths := openAPIMap(t, document, "paths")
	for path, method := range map[string]string{
		"/api/v1/admin/auth/login":         "post",
		"/api/v1/admin/auth/refresh":       "post",
		"/api/v1/admin/auth/logout":        "post",
		"/api/v1/admin/users":              "get",
		"/api/v1/admin/users/{id}":         "get",
		"/api/v1/admin/users/{id}/disable": "post",
		"/api/v1/admin/users/{id}/enable":  "post",
	} {
		operations := openAPIMap(t, paths, path)
		if _, ok := operations[method]; !ok {
			t.Fatalf("admin path %s does not document %s", path, method)
		}
	}

	components := openAPIMap(t, document, "components")
	schemas := openAPIMap(t, components, "schemas")
	for _, schema := range []string{"AdminLoginInput", "AdminTokenData", "AdminUserListData", "AdminUserDetailData", "AdminUserStatusData", "AdminUserStats", "AdminSafeUserProfile"} {
		if _, ok := schemas[schema]; !ok {
			t.Fatalf("missing admin schema %q", schema)
		}
	}
	listData := openAPIMap(t, schemas, "AdminUserListData")
	listProperties := openAPIMap(t, listData, "properties")
	for _, property := range []string{"items", "page", "page_size", "total", "stats"} {
		if _, ok := listProperties[property]; !ok {
			t.Fatalf("admin list data is missing %q", property)
		}
	}
	profile := openAPIMap(t, schemas, "AdminUserListItem")
	profileProperties := openAPIMap(t, profile, "properties")
	for _, property := range []string{"nickname", "avatar", "nickname_moderation_status", "avatar_moderation_status"} {
		if _, ok := profileProperties[property]; !ok {
			t.Fatalf("safe admin profile is missing %q", property)
		}
	}
	users := openAPIMap(t, paths, "/api/v1/admin/users")
	listOperation := openAPIMap(t, users, "get")
	if _, ok := listOperation["security"]; !ok {
		t.Fatal("admin user list must document bearer authentication")
	}
}

func openAPIMap(t *testing.T, values map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := values[key]
	if !ok {
		t.Fatalf("OpenAPI document is missing %q", key)
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("OpenAPI %q has type %T, want object", key, value)
	}
	return mapped
}
