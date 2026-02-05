package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	baseURL := "http://localhost:8080/api"
	apiKey := "$2a$10$HobVM5EhymDPo1sjWFWy2OObqA0wY5Bql947fg37aOarQSEVS8mcG"

	var successCount int64
	var errorCount int64
	var wg sync.WaitGroup

	numRequests := 1000
	concurrentWorkers := 50

	startTime := time.Now()

	jobs := make(chan int, numRequests)
	results := make(chan bool, numRequests)

	// start workers
	for w := 0; w < concurrentWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, baseURL, apiKey, &wg)
	}

	// send jobs
	for j := 0; j < numRequests; j++ {
		jobs <- j
	}
	close(jobs)

	wg.Wait()
	close(results)

	for result := range results {
		if result {
			atomic.AddInt64(&successCount, 1)
		} else {
			atomic.AddInt64(&errorCount, 1)
		}
	}

	duration := time.Since(startTime)
	requestsPerSecond := float64(numRequests) / duration.Seconds()

	fmt.Println("Load Test Results:")
	fmt.Println("==================")
	fmt.Printf("Total Requests: %d\n", numRequests)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", errorCount)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Requests/sec: %.2f\n", requestsPerSecond)
	fmt.Printf("Success Rate: %.2f%%\n",
		float64(successCount)/float64(numRequests)*100)
}

func worker(
	id int,
	jobs <-chan int,
	results chan<- bool,
	baseURL, apiKey string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for range jobs {
		payload := map[string]string{
			"endpoint": "/api/logs",
		}

		jsonData, _ := json.Marshal(payload)

		req, err := http.NewRequest(
			"POST",
			baseURL+"/logs",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			results <- false
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Worker %d error: %v\n", id, err)
			results <- false
			continue
		}

		success := resp.StatusCode >= 200 && resp.StatusCode < 300
		resp.Body.Close()

		results <- success

		time.Sleep(10 * time.Millisecond)
	}
}
