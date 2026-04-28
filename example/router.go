package example

import (
	"github.com/enzujp/walker/internal"
	"github.com/enzujp/walker/internal/registry"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("alrighty"))
}

func SetupRouter() *chi.Mux {
	r := chi.NewRouter()
	RegisterRoute(r, internal.RouteMeta{
		Method: "POST",
		Path:   "/users",
		Req:    CreateUserRequest{},
	}, CreateUserHandler)
	return r
}

func RegisterRoute(r chi.Router, meta internal.RouteMeta, handler http.HandlerFunc) {
	registry.Register(meta)
	r.Method(meta.Method, meta.Path, handler)
}
