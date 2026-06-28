package main

import (
	"net/http"

	"github.com/Meowizz/metrics-collector/internal/handler"
	"github.com/Meowizz/metrics-collector/internal/repository"
)

func main() {
	storage := repository.NewMemStorage()
	handler := handler.NewMetricsHandler(storage)
	mux := http.NewServeMux()
	mux.HandleFunc("/update", handler.UpdatePage)
	mux.HandleFunc("/update/", handler.UpdatePage)
	err := http.ListenAndServe(":8080", mux)

	if err != nil {
		panic(err)
	}
}
