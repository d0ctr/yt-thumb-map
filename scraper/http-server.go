package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"
)

type Server struct {
	rdb       *redis.Client
	channelId string
}

func CreateServer(rdb *redis.Client, channelId string) *Server {
	return &Server{rdb: rdb, channelId: channelId}
}

// Handler function for the GET endpoint
func (Server *Server) getTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the data from Redis by key
	hash, err := Server.rdb.HGetAll(r.Context(), fmt.Sprintf("yt-data:%s", Server.channelId)).Result()
	if err != nil {
		// Redis key not found or other error occurred
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Serialize the Template struct into a JSON response
	response, err := json.Marshal(hash)
	if err != nil {
		// Error occurred while serializing the template data
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")

	// Write the response
	w.Write(response)
}

func (Server *Server) Start(port string) {
	// Set up the HTTP server and endpoint
	http.HandleFunc("/api/get-templates", Server.getTemplatesHandler)

	fs := http.FileServer(http.Dir("../static"))
	http.Handle("/", fs)

	log.Printf("Starting http server at localhost:%s", port)
	// Start the HTTP server as a goroutine
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
