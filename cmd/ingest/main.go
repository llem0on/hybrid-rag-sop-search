package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	dir := flag.String("dir", "", "folder with PDF files to upload")
	server := flag.String("server", "http://localhost:8080", "search service base URL")
	flag.Parse()
	if *dir == "" {
		log.Fatal("usage: ingest -dir ./pdfs [-server http://localhost:8080]")
	}

	files, err := filepath.Glob(filepath.Join(*dir, "*.pdf"))
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	ok := 0
	for _, path := range files {
		if err := upload(client, *server, path); err != nil {
			log.Printf("FAIL %s: %v", filepath.Base(path), err)
			continue
		}
		ok++
		log.Printf("OK   %s", filepath.Base(path))
	}
	fmt.Printf("uploaded %d of %d files\n", ok, len(files))
}

func upload(client *http.Client, server, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	mw.WriteField("title", strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if err := mw.Close(); err != nil {
		return err
	}

	resp, err := client.Post(strings.TrimRight(server, "/")+"/documents", mw.FormDataContentType(), &body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
