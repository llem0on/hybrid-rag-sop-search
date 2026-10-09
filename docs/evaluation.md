# Evaluation

`cmd/eval` sends every query in a labeled dataset to a running server and reports precision, recall, MRR and leaks.

## Run

```bash
make run                                   # in one terminal
make eval DATASET=path/to/dataset.json     # in another
```

Example output on a 244-query dataset:

```text
queries: 244 (180 answerable, 64 negative), documents shown: 188

                       precision    recall
answers                    0.931     0.926
answers + related          0.952     0.877

negative queries with results: 0 / 64
```

Add `-v` to print every query with the titles it returned.

## Dataset format

```json
[
  {
    "query": "who approves a new user account",
    "expected": [
      { "title": "User Access Management", "label": "answer" },
      { "title": "Password Policy", "label": "related" }
    ]
  },
  { "query": "weather forecast for tomorrow", "expected": [] }
]
```

| Label | Meaning |
|---|---|
| `answer` | Document that answers the question |
| `related` | Document a reader would reasonably also want |
| *(empty list)* | Out-of-scope query; any result counts as a leak |

Titles are matched case-insensitively against `document.title`.

## Metrics

| Metric | Definition |
|---|---|
| Precision (answers) | answer documents shown ÷ all documents shown |
| Recall (answers) | answer documents shown ÷ all answer labels |
| Precision / recall (answers + related) | same, counting both labels as correct |
| MRR | mean of 1 / rank of the first answer, over answerable queries |
| Leaks | negative queries that returned at least one document |

## Building a good dataset

- Mix query styles: titles, questions, paraphrases, typos, English variants of Indonesian documents.
- Add negatives on unrelated topics **and** trap queries that share words with your documents but ask something else.
- Label `related` strictly: only documents about the same task, not the same broad topic. Loose labels make recall look worse than it is.
- Use at least 100 queries before comparing settings; with fewer, one query moves the numbers by a whole percent.
