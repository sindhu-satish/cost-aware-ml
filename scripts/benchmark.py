import requests
import time

gateway_url = "http://localhost:8080"
requests_count = 100

print(f"Sending {requests_count} requests...")

for i in range(1, requests_count + 1):
    if i % 10 < 3:
        budget = (i % 20) / 10.0
    elif i % 10 < 7:
        budget = 3.0 + (i % 50) / 10.0
    else:
        budget = 8.0 + (i % 70) / 10.0

    payload = {
        "request_id": f"bench-{i}",
        "user_id": f"user-{i % 10}",
        "tenant_id": "tenant-1",
        "input": f"test input {i} with varying length {'x' * (i % 100)}",
        "budget": budget
    }
    
    try:
        requests.post(f"{gateway_url}/infer", json=payload, timeout=10)
    except:
        pass
    
    if i % 10 == 0:
        print(f"Sent {i} requests...")
    time.sleep(0.05)

