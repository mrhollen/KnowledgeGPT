package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/mrhollen/KnowledgeGPT/internal/parsing"
)

type UploadHandler struct{}

// uploadHandler handles the /upload POST endpoint
func (u *UploadHandler) UploadFile(userId int64, w http.ResponseWriter, r *http.Request) {
	// Explicitly set the maximum upload size to 10MB
	const MaxUploadSize = 10 * 1024 * 1024 // 10 MB

	// Set MaxBytesReader before any other operations to limit the request size
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)

	// Log the Content-Length if available
	if cl := r.Header.Get("Content-Length"); cl != "" {
		if size, err := strconv.Atoi(cl); err == nil {
			log.Printf("Incoming request size: %d bytes", size)
		}
	}

	// Parse the multipart form with a buffer of MaxUploadSize
	err := r.ParseMultipartForm(MaxUploadSize)
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		http.Error(w, "The uploaded file is too big. Please choose a file that's less than 10MB in size", http.StatusBadRequest)
		return
	}

	// Retrieve the file from form data
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error retrieving the file: %v", err)
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check the file extension
	if !parsing.IsPDF(header.Filename) {
		log.Printf("Invalid file type uploaded: %s", header.Filename)
		http.Error(w, "Please upload a PDF file", http.StatusBadRequest)
		return
	}

	// Read the file into a buffer
	var buf bytes.Buffer
	n, err := io.Copy(&buf, file)
	if err != nil {
		log.Printf("Error reading the file: %v", err)
		http.Error(w, "Failed to read uploaded file", http.StatusInternalServerError)
		return
	}
	log.Printf("Uploaded file size: %d bytes", n)

	// Extract text from PDF using the pdfparser package
	text, err := parsing.ExtractTextFromPDF(buf.Bytes())
	if err != nil {
		log.Printf("Error extracting text: %v", err)
		http.Error(w, "Failed to extract text from PDF", http.StatusInternalServerError)
		return
	}

	// Return the extracted text
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(text))
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
