import os
import time
from fastapi import FastAPI

app = FastAPI()

@app.get("/healthz")
def health():
    return {"status": "ok", "service": "tier2_best"}

@app.post("/infer")
def infer(request: dict):
    time.sleep(0.250)  
    return {
        "result": "prediction_tier2",
        "confidence": 0.96,
        "model_latency_ms": 250,
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8092"))
    uvicorn.run(app, host="0.0.0.0", port=port)

