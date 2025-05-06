package adapter

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type HttpAdapter interface {
	SetHeaders(w http.ResponseWriter)
	HandleExternalAPICall(w http.ResponseWriter, r *http.Request)
}

type StreamingAdapter struct {
	ExternalAPI string
	Client      *http.Client
	NumWorkers  int
	NumRequests int
}

func NewStreamingAdapter(ExternalAPI string) *StreamingAdapter {
	return &StreamingAdapter{
		ExternalAPI: ExternalAPI,
		Client:      &http.Client{Timeout: 30 * time.Second},
		NumWorkers:  1,
		NumRequests: 20,
	}
}

func (s StreamingAdapter) SetHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

func (s StreamingAdapter) CheckStreamingSupport(w http.ResponseWriter) (http.Flusher, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return nil, fmt.Errorf("streaming unsupported")
	}
	return flusher, nil
}

func (s StreamingAdapter) HandleExternalAPICall(w http.ResponseWriter, r *http.Request) {
	fmt.Println("New stream client connected")
	s.SetHeaders(w)

	flusher, err := s.CheckStreamingSupport(w)
	if err != nil {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
			case <-done:
				return
			}
		}
	}()

	for i := 0; i < s.NumRequests; i++ {
		url := fmt.Sprintf("%s%d", s.ExternalAPI, i)
		resp, err := s.Client.Get(url)
		if err != nil {
			fmt.Fprintf(w, "data: Error %d: %v\n\n", i, err)
			flusher.Flush()
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		fmt.Fprintf(w, "data: %s\n\n", string(body))
		flusher.Flush()
	}

	ticker.Stop()
	close(done)
}

type PlainAdapter struct {
	ExternalAPI string
	Client      *http.Client
	NumRequests int
}

func NewPlainAdapter(ExternalAPI string) *PlainAdapter {
	return &PlainAdapter{
		ExternalAPI: ExternalAPI,
		Client:      &http.Client{Timeout: 30 * time.Second},
		NumRequests: 20,
	}
}

func (p PlainAdapter) SetHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (p PlainAdapter) HandleExternalAPICall(w http.ResponseWriter, r *http.Request) {
	p.SetHeaders(w)

	var results []string
	for i := 0; i < p.NumRequests; i++ {
		url := fmt.Sprintf("%s%d", p.ExternalAPI, i)
		resp, err := p.Client.Get(url)
		if err != nil {
			results = append(results, fmt.Sprintf("Error %d: %v", i, err))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		results = append(results, string(body))
	}

	fmt.Fprint(w, "[\""+fmt.Sprint(results)+"\"]")
}
