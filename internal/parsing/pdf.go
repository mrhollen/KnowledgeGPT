package parsing

import (
	"bytes"
	"fmt"

	"github.com/ledongthuc/pdf"
)

// ExtractTextFromPDF takes a byte slice of a PDF file and returns the extracted plain text.
func ExtractTextFromPDF(pdfData []byte) (string, error) {
	reader := bytes.NewReader(pdfData)
	pdfReader, err := pdf.NewReader(reader, int64(len(pdfData)))
	if err != nil {
		return "", fmt.Errorf("error creating PDF reader: %v", err)
	}

	var buf bytes.Buffer
	b, err := pdfReader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("could not read content of pdf: %v", err)
	}

	buf.ReadFrom(b)
	return buf.String(), nil
}

// IsPDF checks if the provided filename has a .pdf extension (case-insensitive).
func IsPDF(filename string) bool {
	if len(filename) < 4 {
		return false
	}
	ext := filename[len(filename)-4:]
	switch ext {
	case ".pdf", ".PDF":
		return true
	default:
		return false
	}
}
