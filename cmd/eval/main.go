package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type expected struct {
	Title string `json:"title"`
	Label string `json:"label"`
}

type example struct {
	Query    string     `json:"query"`
	Expected []expected `json:"expected"`
}

type searchResponse struct {
	Results []struct {
		Document struct {
			Title  string `json:"title"`
			Number string `json:"number"`
		} `json:"document"`
	} `json:"results"`
}

type tally struct {
	shown, answers, answersHit, all, allHit, negatives, leaks int
	rr                                                        float64
	positives                                                 int
}

func main() {
	datasetPath := flag.String("dataset", "examples/dataset.example.json", "evaluation dataset (JSON)")
	server := flag.String("server", "http://localhost:8080", "search service base URL")
	verbose := flag.Bool("v", false, "print every query")
	flag.Parse()

	raw, err := os.ReadFile(*datasetPath)
	if err != nil {
		log.Fatal(err)
	}
	var dataset []example
	if err := json.Unmarshal(raw, &dataset); err != nil {
		log.Fatalf("parse dataset: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	var t tally
	for _, ex := range dataset {
		got, err := query(client, *server, ex.Query)
		if err != nil {
			log.Fatalf("query %q: %v", ex.Query, err)
		}
		t.add(ex, got)
		if *verbose {
			fmt.Printf("%-60.60s -> %s\n", ex.Query, strings.Join(got, " | "))
		}
	}
	t.print(len(dataset))
}

func query(client *http.Client, server, q string) ([]string, error) {
	body, _ := json.Marshal(map[string]string{"query": q})
	resp, err := client.Post(strings.TrimRight(server, "/")+"/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sr.Results))
	for _, r := range sr.Results {
		out = append(out, r.Document.Title)
	}
	return out, nil
}

func (t *tally) add(ex example, got []string) {
	t.shown += len(got)
	if len(ex.Expected) == 0 {
		t.negatives++
		if len(got) > 0 {
			t.leaks++
		}
		return
	}
	shown := map[string]bool{}
	for _, g := range got {
		shown[strings.ToLower(g)] = true
	}
	answers := map[string]bool{}
	for _, e := range ex.Expected {
		key := strings.ToLower(e.Title)
		t.all++
		if shown[key] {
			t.allHit++
		}
		if e.Label != "related" {
			answers[key] = true
			t.answers++
			if shown[key] {
				t.answersHit++
			}
		}
	}
	t.positives++
	for i, g := range got {
		if answers[strings.ToLower(g)] {
			t.rr += 1 / float64(i+1)
			break
		}
	}
}

func (t *tally) print(n int) {
	ratio := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return float64(a) / float64(b)
	}
	answerShown := t.answersHit
	allShown := t.allHit
	fmt.Printf("\nqueries: %d (%d answerable, %d negative), documents shown: %d\n\n", n, t.positives, t.negatives, t.shown)
	fmt.Printf("%-22s %9s %9s\n", "", "precision", "recall")
	fmt.Printf("%-22s %9.3f %9.3f\n", "answers", ratio(answerShown, t.shown), ratio(t.answersHit, t.answers))
	fmt.Printf("%-22s %9.3f %9.3f\n", "answers + related", ratio(allShown, t.shown), ratio(t.allHit, t.all))
	fmt.Printf("\nMRR (answers): %.3f\n", t.rr/float64(max(t.positives, 1)))
	fmt.Printf("negative queries with results: %d / %d\n", t.leaks, t.negatives)
}
