import os
import time
from fastapi import FastAPI, Request

app = FastAPI()

@app.get("/healthz")
def health():
    return {"status": "ok", "service": "tier1_mid"}

@app.post("/infer")
async def infer(request: Request):
    await request.json()
    time.sleep(0.085)
    return {
        "result": "prediction_tier1",
        "confidence": 0.88,
        "model_latency_ms": 85,
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8091"))
    uvicorn.run(app, host="0.0.0.0", port=port)

