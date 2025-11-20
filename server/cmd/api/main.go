// package api

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/devrayat000/video-process/utils"
	amqp "github.com/rabbitmq/amqp091-go"
)

// --- Configuration ---
var rabbitURL = os.Getenv("RABBITMQ_URL")

func main() {
	// 1. Connect to RabbitMQ
	// Retry logic is crucial because RabbitMQ might take a few seconds to boot in Docker
	var conn *amqp.Connection
	var err error

	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("Waiting for RabbitMQ... (%s)", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ after retries")
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	// 2. Declare Queue (Ensure it exists)
	q, err := ch.QueueDeclare(
		"video_jobs", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	// 3. HTTP Handlers
	http.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var job utils.VideoJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Publish to RabbitMQ
		body, _ := json.Marshal(job)
		err = ch.Publish(
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})

		if err != nil {
			log.Printf("Failed to publish message: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		log.Printf(" [x] Sent Job: %s", job.VideoID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "queued", "id": job.VideoID})
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "api_server"})
	})

	log.Println("API Server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}
