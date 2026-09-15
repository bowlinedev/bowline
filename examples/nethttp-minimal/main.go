package main

import (
	"context"
	"log"
	"net/http"

	"github.com/bowlinedev/bowline"
)

type GreetInput struct {
	Name string `json:"name" validate:"required,min=1,max=40"`
}

type Greeting struct {
	Message string `json:"message"`
}

func Greet(ctx context.Context, in GreetInput) (Greeting, error) {
	return Greeting{Message: "hello, " + in.Name}, nil
}

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("greet", Greet))
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/api/", Routes().Handler())
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
