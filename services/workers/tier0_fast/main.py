import os
import time
import hashlib
from fastapi import FastAPI, Request

app = FastAPI()

def calculate_confidence(input_data):
    if isinstance(input_data, str):
        input_len = len(input_data)
        base_confidence = 0.72
        
        if input_len < 10:
            return min(0.95, base_confidence + 0.15)
        elif input_len < 50:
            return base_confidence
        elif input_len < 100:
            return max(0.50, base_confidence - 0.10)
        else:
            return max(0.45, base_confidence - 0.20)
    return 0.72

@app.get("/healthz")
def health():
    return {"status": "ok", "service": "tier0_fast"}

@app.post("/infer")
async def infer(request: Request):
    data = await request.json()
    input_data = data.get("input", "")
    
    confidence = calculate_confidence(input_data)
    
    time.sleep(0.015)
    
    input_hash = hashlib.md5(str(input_data).encode()).hexdigest()[:8]
    
    return {
        "result": f"prediction_tier0_{input_hash}",
        "confidence": round(confidence, 2),
        "model_latency_ms": 15,
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8090"))
    uvicorn.run(app, host="0.0.0.0", port=port)
