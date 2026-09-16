package example

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enzujp/walker/pkg/walker"
)

func TestMinimalDocumentedRouter(t *testing.T) {
	api := walker.NewRouter()
	api.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	api.Route("/users", func(users *walker.Router) {
		users.Get("/{id}", func(http.ResponseWriter, *http.Request) {}, walker.PathValue("id", "42"))
		users.Post("/", func(http.ResponseWriter, *http.Request) {}, walker.Body(CreateUserRequest{
			Name: "Ada", Email: "ada@example.com",
		}))
	}, walker.Group("Users"))

	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("health status: %d", response.Code)
	}
	collection, err := api.Postman(walker.Options{
		Name: "Users API",
		Auth: walker.BearerAuth("{{token}}"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range [][]byte{[]byte(`"GET /health"`), []byte(`"GET /users/{id}"`), []byte(`"POST /users/"`), []byte(`ada@example.com`)} {
		if !bytes.Contains(collection, expected) {
			t.Fatalf("collection does not contain %s:\n%s", expected, collection)
		}
	}
}
