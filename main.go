package main

import (
	adapter "HttpAdapter/Adapter"
	"fmt"
	"net/http"
	"time"
)

// Simulates fetching an item with delay
func mockAPIHandler(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Path[len("/api/item/"):]
    time.Sleep(2 * time.Second)
    fmt.Fprintf(w, "This is item %s", id)
}


func main() {
	url := "http://localhost:8080/api/item/"
	streaming := adapter.NewStreamingAdapter(url)
	plain := adapter.NewPlainAdapter(url)

	http.HandleFunc("/stream", streaming.HandleExternalAPICall)
	http.HandleFunc("/batch", plain.HandleExternalAPICall)
	http.HandleFunc("/api/item/", mockAPIHandler)
	http.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("Server listening on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}


