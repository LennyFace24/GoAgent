# Milvus Lite Service

轻量级向量数据库服务，基于 Milvus Lite + FastAPI。

## 安装

```bash
cd milvus-service
pip install -r requirements.txt
```

## 启动

```bash
python main.py
```

服务默认运行在 `http://localhost:8088`

## API 接口

### 健康检查
```bash
GET /health
```

### 创建 Collection
```bash
POST /collection/create
{
    "collection_name": "documents",
    "dimension": 1536,
    "metric_type": "COSINE"
}
```

### 插入向量
```bash
POST /insert
{
    "collection_name": "documents",
    "ids": ["doc1", "doc2"],
    "vectors": [[0.1, 0.2, ...], [0.3, 0.4, ...]],
    "documents": ["文本1", "文本2"]
}
```

### 搜索
```bash
POST /search
{
    "collection_name": "documents",
    "vector": [0.1, 0.2, ...],
    "top_k": 5,
    "output_fields": ["document"]
}
```

### 删除
```bash
POST /delete
{
    "collection_name": "documents",
    "ids": ["doc1"]
}
```

### 删除 Collection
```bash
DELETE /collection/{collection_name}
```
