import os
import time
import hashlib
from fastapi import FastAPI, Request

app = FastAPI()

def calculate_confidence(input_data):
    if isinstance(input_data, str):
        input_len = len(input_data)
        base_confidence = 0.88
        
        if input_len < 10:
            return min(0.98, base_confidence + 0.08)
        elif input_len < 50:
            return base_confidence
        elif input_len < 100:
            return max(0.75, base_confidence - 0.10)
        else:
            return max(0.70, base_confidence - 0.15)
    return 0.88

@app.get("/healthz")
def health():
    return {"status": "ok", "service": "tier1_mid"}

@app.post("/infer")
async def infer(request: Request):
    data = await request.json()
    input_data = data.get("input", "")
    
    confidence = calculate_confidence(input_data)
    
    time.sleep(0.085)
    
    input_hash = hashlib.md5(str(input_data).encode()).hexdigest()[:8]
    
    return {
        "result": f"prediction_tier1_{input_hash}",
        "confidence": round(confidence, 2),
        "model_latency_ms": 85,
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8091"))
    uvicorn.run(app, host="0.0.0.0", port=port)
