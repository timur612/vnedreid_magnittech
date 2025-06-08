from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List
import httpx  # type: ignore
from ollama import AsyncClient, ResponseError, Client
from utils import preprocess_metrics


class MetricsRequest(BaseModel):
    cluster: str
    pod: str
    cpu_data: List[float]
    ram_data: List[float]
    cpu_cost: float
    ram_cost: float


app = FastAPI()
# client = AsyncClient(
#     host="http://localhost:11434",
#     headers={"Content-Type": "application/json"},
#     timeout=60.0,
#     follow_redirects=True,
# )

client = Client(host="http://localhost:11434")


@app.post("/get_llm_rec")
async def get_llm_rec(request: MetricsRequest):
    cpu_data, ram_data = preprocess_metrics(request.cpu_data, request.ram_data)
    prompt = f"""
Проанализируй текущие значения загрузки для пода {request.pod} в кластере {request.cluster}:
 - CPU: {cpu_data}
 - RAM: {ram_data}
Укажи, что не так с подами и предложи, что можно сделать для исправления.
Отвечай краткою"""
    try:
        response = client.chat(model="gemma3:12b",
                               messages=[{"role": "system", "content": "Ты эксперт по оптимизации Kubernetes-кластеров."},
                                         {"role": "user", "content": prompt}])
    except (
            ConnectionError, ResponseError,
            httpx.RemoteProtocolError) as e:  # type: ignore
        raise HTTPException(status_code=502, detail=f"Ollama error: {e}")
    content = response.message.content
    return {"recommendation": content}
