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
	ExternalAPI   string
	Client        *http.Client
	NumWorkers    int
	NumRequests   int
	Job           struct{ Index int }
	CollectedStreamData struct {
		Index int
		Data  string
	}
}

func NewStreamingAdapter(ExternalAPI string) *StreamingAdapter {
	// Timeout: 30 * time.Second
	return &StreamingAdapter{
		ExternalAPI: ExternalAPI,
		Client:      &http.Client{},
		NumWorkers:  4,
		NumRequests: 20,
	}
}

func (s *StreamingAdapter) SetHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

func (s *StreamingAdapter) CheckStreamingSupport(w http.ResponseWriter) (http.Flusher, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return nil, fmt.Errorf("streaming unsupported")
	}
	return flusher, nil
}

func (s *StreamingAdapter) HandleExternalAPICall(w http.ResponseWriter, r *http.Request) {
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

	// Channels
	jobs := make(chan struct{ Index int }, s.NumRequests)
	CollectedStreamDatas := make(chan struct {
		Index int
		Data  string
	}, s.NumRequests)

	// Workers: Process jobs concurrently, but stream results one by one
	for w := 0; w < s.NumWorkers; w++ {
		go func() {
			for job := range jobs {
				url := fmt.Sprintf("%s%d", s.ExternalAPI, job.Index)
				resp, err := s.Client.Get(url)
				var data string
				if err != nil {
					data = fmt.Sprintf("Error %d: %v", job.Index, err)
				} else {
					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					data = string(body)
				}
				// Send the result to the channel for streaming
				CollectedStreamDatas <- struct {
					Index int
					Data  string
				}{Index: job.Index, Data: data}
			}
		}()
	}

	// Send jobs to workers
	go func() {
		for i := 0; i < s.NumRequests; i++ {
			jobs <- struct{ Index int }{Index: i}
		}
		close(jobs)
	}()

	// Collect and stream data in order
	for i := 0; i < s.NumRequests; i++ {
		// Wait for the next result from the workers
		CollectedStreamData := <-CollectedStreamDatas

		// Stream the collected data one by one
		fmt.Fprintf(w, "data: %s\n\n", CollectedStreamData.Data)
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

	var CollectedStreamDatas []string
	for i := 0; i < p.NumRequests; i++ {
		url := fmt.Sprintf("%s%d", p.ExternalAPI, i)
		resp, err := p.Client.Get(url)
		if err != nil {
			CollectedStreamDatas = append(CollectedStreamDatas, fmt.Sprintf("Error %d: %v", i, err))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		CollectedStreamDatas = append(CollectedStreamDatas, string(body))
	}

	fmt.Fprint(w, "[\""+fmt.Sprint(CollectedStreamDatas)+"\"]")
}
