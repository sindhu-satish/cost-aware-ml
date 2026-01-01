import os
import time
import hashlib
from fastapi import FastAPI, Request

app = FastAPI()

def calculate_confidence(input_data):
    if isinstance(input_data, str):
        input_len = len(input_data)
        base_confidence = 0.96
        
        if input_len < 10:
            return min(0.99, base_confidence + 0.02)
        elif input_len < 50:
            return base_confidence
        elif input_len < 100:
            return max(0.90, base_confidence - 0.05)
        else:
            return max(0.88, base_confidence - 0.06)
    return 0.96

@app.get("/healthz")
def health():
    return {"status": "ok", "service": "tier2_best"}

@app.post("/infer")
async def infer(request: Request):
    data = await request.json()
    input_data = data.get("input", "")
    
    confidence = calculate_confidence(input_data)
    
    time.sleep(0.250)
    
    input_hash = hashlib.md5(str(input_data).encode()).hexdigest()[:8]
    
    return {
        "result": f"prediction_tier2_{input_hash}",
        "confidence": round(confidence, 2),
        "model_latency_ms": 250,
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8092"))
    uvicorn.run(app, host="0.0.0.0", port=port)
