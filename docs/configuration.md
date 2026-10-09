# Configuration

All settings are environment variables. Copy `.env.example` to `.env`; `make run` loads it.

## Service

| Variable | Default | Description |
|---|---|---|
| `ADDR` | `:8080` | Listen address |
| `DATA_DIR` | `./data` | Index file, uploaded PDFs and figure images |

## Models

| Variable | Default | Description |
|---|---|---|
| `LLM_BASE_URL` | *(empty)* | OpenAI-compatible base URL ending in `/v1`. Empty runs keyword mode |
| `LLM_API_KEY` | *(empty)* | Sent as `Authorization: Bearer` |
| `EMBED_MODEL` | `qwen3-vl-embedding-8b` | Model for `/embeddings` |
| `RERANK_MODEL` | *(empty)* | Model for `/rerank`. Empty disables reranking |
| `RERANK_URL` | `{LLM_BASE_URL without /v1}/rerank` | Override when the reranker lives elsewhere |
| `DOC_INSTRUCTION` | `Represent the user's input.` | System instruction for documents |
| `QUERY_INSTRUCTION` | `Retrieve relevant documents for the query.` | System instruction for queries |
| `EMBED_TIMEOUT` | `60s` | Per embedding request while indexing |
| `RERANK_TIMEOUT` | `25s` | Per rerank request |

Any server that speaks the OpenAI `/v1/embeddings` format and a Cohere/Jina-style `/rerank` works, for example LiteLLM, vLLM, Infinity or TEI.

## OCR

| Variable | Default | Description |
|---|---|---|
| `OCR_ENABLED` | `true` | Extract and OCR figures inside PDFs |
| `OCR_LANGUAGE` | `eng` | Tesseract language, e.g. `ind+eng` |

## Search

| Variable | Default | Description |
|---|---|---|
| `MIN_SEMANTIC` | `0.60` | Cosine floor for the semantic pass |
| `BM25_MIN` | `12` | BM25 floor for the keyword pass |
| `BM25_MIN_COVERAGE` | `0.5` | Share of query terms required for the keyword pass |
| `W_SEMANTIC` | `0.8` | Vector lane weight in fusion |
| `W_KEYWORD` | `0.2` | BM25 lane weight in fusion |
| `RRF_K` | `60` | RRF constant |
| `RERANK_MIN` | `0.5` | Rerank floor for rank 2 and below |
| `RERANK_TOP_MIN` | `0.15` | Rerank floor for rank 1 |
| `RERANK_TOP_COS` | `0.65` | Cosine that keeps rank 1 without the rerank floor |
| `MAX_RESULTS` | `4` | Results after the cut |

The defaults were calibrated for Qwen3-VL-Embedding-8B and Qwen3-VL-Reranker-8B on a corpus of about 470 chunks. Cosine ranges differ between models, so recalibrate `MIN_SEMANTIC` and `RERANK_TOP_COS` with [the evaluation tool](evaluation.md) when you switch models.
