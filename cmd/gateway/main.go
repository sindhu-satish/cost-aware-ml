package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

var controlplaneURL = os.Getenv("CONTROLPLANE_URL")
var tier0URL = os.Getenv("TIER0_URL")
var tier1URL = os.Getenv("TIER1_URL")
var tier2URL = os.Getenv("TIER2_URL")

func main() {
	if controlplaneURL == "" {
		controlplaneURL = "http://controlplane:8081"
	}
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
		port = "8080"
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "gateway"})
	})

	http.HandleFunc("/infer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		decideReq, _ := json.Marshal(req)
		resp, err := http.Post(controlplaneURL+"/decide", "application/json", bytes.NewBuffer(decideReq))
		if err != nil {
			http.Error(w, "controlplane error", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var decision map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&decision)

		tier, _ := decision["tier"].(string)
		var workerURL string
		switch tier {
		case "tier0":
			workerURL = tier0URL
		case "tier1":
			workerURL = tier1URL
		case "tier2":
			workerURL = tier2URL
		default:
			http.Error(w, "unknown tier", http.StatusInternalServerError)
			return
		}

		workerResp, err := http.Post(workerURL+"/infer", "application/json", bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "worker error", http.StatusInternalServerError)
			return
		}
		defer workerResp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, workerResp.Body)
	})

	log.Printf("gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

