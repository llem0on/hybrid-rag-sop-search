# Optional: n8n Workflows

The service calls the model endpoint directly by default (`LLM_BASE_URL`). Routing embeddings through n8n is an optional approach.

| Benefit | Why it helps |
|---|---|
| **Logs out of the box** | Every call appears in n8n's execution dashboard with input, output, duration and error. Failed or slow calls can be inspected and re-run from the UI, with no logging code in the backend |
| **Template outside the code** | The prompt template is edited in n8n, so it changes without a backend redeploy |
| **One workflow, several environments** | The callback URL is chosen from an `env` field, so development and production share the same workflow |

The trade-off is one more service to run and an extra network hop per call.

## Embed Documents

<p align="center">
  <img src="../assets/n8n-embed-documents.svg" width="880" alt="Webhook, Split Out, Embed, Save Embedding">
</p>

Runs **asynchronously**. The backend hands over a batch of chunks and returns immediately; vectors arrive one by one through a callback.

| Node | Does |
|---|---|
| **Webhook** | Receives `{ document_id, env, data: [ { chunk_id, text } ] }` |
| **Split Out** | Turns `data` into one item per chunk, so each chunk is embedded and saved on its own |
| **Embed** | `POST /v1/embeddings` with the chat template. Figure chunks are sent as their OCR text |
| **Save Embedding** | `POST` back to the backend: `{ chunk_id, embedding, model }`. The callback URL is picked from `env`, so development and production share one workflow |

```mermaid
sequenceDiagram
    participant BE as Backend
    participant N as n8n
    participant M as Model server
    BE->>BE: store chunks with empty vectors (pending)
    BE->>N: POST chunks
    N-->>BE: 200 accepted
    loop each chunk
        N->>M: embeddings
        M-->>N: vector
        N->>BE: callback chunk_id + vector
        BE->>BE: save vector
    end
    BE->>BE: document ready when no chunk is pending
```

A chunk with an empty vector doubles as the work queue: a failed callback simply leaves the chunk pending, and re-sending pending chunks retries only what is missing.

## Embed Query

<p align="center">
  <img src="../assets/n8n-embed-query.svg" width="880" alt="Webhook, Embed Query, Respond to Webhook">
</p>

Runs **synchronously**, because a search waits for its vector.

| Node | Does |
|---|---|
| **Webhook** | Receives `{ text }`, the normalized query |
| **Embed Query** | `POST /v1/embeddings` with the same template as documents |
| **Respond to Webhook** | Returns the OpenAI-style response `{ data: [ { embedding } ] }` |

## Prompt template

Both workflows wrap the text the same way. Query and document vectors are only comparable when they come from the same template.

```text
<|im_start|>system
{instruction}
<|im_end|>
<|im_start|>user
{text}
<|im_end|>
<|im_start|>assistant
```

As an n8n expression in the **Embed Query** node:

```js
{{ JSON.stringify({
  model: "qwen3-vl-embedding-8b",
  input: "<|im_start|>system\n" + $vars.QUERY_INSTRUCTION + "\n<|im_end|>\n<|im_start|>user\n"
       + $json.body.text + "\n<|im_end|>\n<|im_start|>assistant\n"
}) }}
```

Changing the instruction in one workflow means changing it in the other and re-embedding every document.
