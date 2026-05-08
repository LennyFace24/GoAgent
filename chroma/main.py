from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import chromadb
from typing import List, Optional
import uvicorn

app = FastAPI(title="ChromaDB Vector Service")

# ChromaDB 客户端（持久化存储）
client = chromadb.PersistentClient(path="./chroma_data")


class CreateCollectionRequest(BaseModel):
    collection_name: str


class InsertRequest(BaseModel):
    collection_name: str
    ids: List[str]
    embeddings: List[List[float]]
    documents: Optional[List[str]] = None
    metadatas: Optional[List[dict]] = None


class SearchRequest(BaseModel):
    collection_name: str
    query_embeddings: List[List[float]]
    n_results: int = 5
    include: Optional[List[str]] = ["documents", "distances", "metadatas"]


class DeleteRequest(BaseModel):
    collection_name: str
    ids: List[str]


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/collection/create")
async def create_collection(req: CreateCollectionRequest):
    try:
        collection = client.get_or_create_collection(
            name=req.collection_name,
            metadata={"hnsw:space": "cosine"}
        )
        return {
            "status": "ok",
            "collection": req.collection_name,
            "count": collection.count()
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.delete("/collection/{collection_name}")
async def delete_collection(collection_name: str):
    try:
        client.delete_collection(name=collection_name)
        return {"status": "deleted", "collection": collection_name}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/collection/{collection_name}/exists")
async def collection_exists(collection_name: str):
    try:
        collections = [c.name for c in client.list_collections()]
        exists = collection_name in collections
        return {"collection": collection_name, "exists": exists}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/insert")
async def insert(req: InsertRequest):
    try:
        collection = client.get_collection(name=req.collection_name)

        kwargs = {
            "ids": req.ids,
            "embeddings": req.embeddings,
        }
        if req.documents:
            kwargs["documents"] = req.documents
        if req.metadatas:
            kwargs["metadatas"] = req.metadatas

        collection.add(**kwargs)
        return {"status": "inserted", "count": len(req.ids)}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/search")
async def search(req: SearchRequest):
    try:
        collection = client.get_collection(name=req.collection_name)

        results = collection.query(
            query_embeddings=req.query_embeddings,
            n_results=req.n_results,
            include=req.include
        )
        return {"status": "ok", "results": results}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/delete")
async def delete(req: DeleteRequest):
    try:
        collection = client.get_collection(name=req.collection_name)
        collection.delete(ids=req.ids)
        return {"status": "deleted", "count": len(req.ids)}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8088)
