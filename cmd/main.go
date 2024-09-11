package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type NodeResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Data     bool        `json:"data"`
		Metadata interface{} `json:"metadata"`
	} `json:"result"`
	ID int `json:"id"`
}

func checkNodeHealth() bool {
	url := "http://node:8648"
	requestBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "isConsensusEstablished",
		"params":  []interface{}{},
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Println("Error making request to node:", err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		return false
	}

	var nodeResponse NodeResponse
	err = json.Unmarshal(body, &nodeResponse)
	if err != nil {
		log.Println("Error unmarshaling response:", err)
		return false
	}

	return nodeResponse.Result.Data
}

func healthCheck(c *gin.Context) {
	if checkNodeHealth() {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy"})
	}
}

func main() {
	r := gin.Default()
	r.GET("/health", healthCheck)
	r.Run(":8080")
}
