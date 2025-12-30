package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

var tier0URL = os.Getenv("TIER0_URL")
var tier1URL = os.Getenv("TIER1_URL")
var tier2URL = os.Getenv("TIER2_URL")

func main() {
	if tier0URL == "" {
		tier0URL = "http://tier0-fast:8090"
	}
	if tier1URL == "" {
		tier1URL = "http://tier1-mid:8091"
	}
	if tier2URL == "" {
		tier2URL = "http://tier2-best:8092"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "controlplane"})
	})

	http.HandleFunc("/decide", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req map[string]interface{}
		json.Unmarshal(body, &req)

		tier := "tier0"
		if budget, ok := req["budget"].(float64); ok && budget > 5 {
			tier = "tier1"
		}
		if budget, ok := req["budget"].(float64); ok && budget > 10 {
			tier = "tier2"
		}

		workerURL := tier0URL
		if tier == "tier1" {
			workerURL = tier1URL
		}
		if tier == "tier2" {
			workerURL = tier2URL
		}

		workerResp, err := http.Post(workerURL+"/infer", "application/json", bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "worker error", http.StatusInternalServerError)
			return
		}
		defer workerResp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(workerResp.Body).Decode(&result)
		result["tier"] = tier

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Printf("controlplane listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

