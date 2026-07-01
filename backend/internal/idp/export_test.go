package idp

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExportDocumentFormats(t *testing.T) {
	current := 2
	doc := ExportDocument{Plan: &Plan{Title: "Развитие", EmployeeName: "Иван Иванов", ManagerName: "Анна Смирнова", StartDate: "2026-01-01", EndDate: "2026-06-30", Status: "active", Progress: 40, Competencies: []CompetencyTarget{{Name: "Go", CurrentLevel: &current, TargetLevel: 3}}}, Tasks: []ExportTask{{Title: "Курс", Status: "in_progress", Priority: "high", Progress: 40}}}

	xlsx, err := doc.XLSX()
	if err != nil {
		t.Fatalf("XLSX: %v", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(xlsx))
	if err != nil {
		t.Fatalf("open XLSX: %v", err)
	}
	defer book.Close()
	if value, _ := book.GetCellValue("ИПР", "B1"); value != "Развитие" {
		t.Fatalf("unexpected title: %q", value)
	}

	pdf, err := doc.PDF()
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("invalid PDF signature")
	}
}
