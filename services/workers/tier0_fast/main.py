import os
import time
from fastapi import FastAPI, Request

app = FastAPI()

@app.get("/healthz")
def health():
    return {"status": "ok", "service": "tier0_fast"}

@app.post("/infer")
async def infer(request: Request):
    await request.json()
    time.sleep(0.015)
    return {
        "result": "prediction_tier0",
        "confidence": 0.72,
        "model_latency_ms": 15,
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8090"))
    uvicorn.run(app, host="0.0.0.0", port=port)

