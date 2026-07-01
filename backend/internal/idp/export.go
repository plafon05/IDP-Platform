package idp

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
	"golang.org/x/image/font/gofont/goregular"
)

type ExportTask struct {
	Title, Category, Priority, DueDate, Status, Rating string
	Progress                                           int
}

type ExportDocument struct {
	Plan  *Plan
	Tasks []ExportTask
}

func (s *Service) Export(ctx context.Context, access Access, id string) (*ExportDocument, error) {
	plan, err := s.Get(ctx, access, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT t.title, COALESCE(c.name, ''), t.priority, COALESCE(t.due_date::text, ''),
			t.status, t.progress, COALESCE(t.manager_rating, '')
		FROM tasks t LEFT JOIN task_categories c ON c.id = t.category_id
		WHERE t.idp_id = $1 AND t.deleted_at IS NULL ORDER BY t.due_date NULLS LAST, t.created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	doc := &ExportDocument{Plan: plan, Tasks: []ExportTask{}}
	for rows.Next() {
		var task ExportTask
		if err := rows.Scan(&task.Title, &task.Category, &task.Priority, &task.DueDate, &task.Status, &task.Progress, &task.Rating); err != nil {
			return nil, err
		}
		doc.Tasks = append(doc.Tasks, task)
	}
	return doc, rows.Err()
}

func (d *ExportDocument) XLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "ИПР"
	f.SetSheetName("Sheet1", sheet)
	rows := [][]any{
		{"Индивидуальный план развития", d.Plan.Title},
		{"Сотрудник", d.Plan.EmployeeName}, {"Руководитель", d.Plan.ManagerName},
		{"Период", d.Plan.StartDate + " - " + d.Plan.EndDate}, {"Статус", statusLabel(d.Plan.Status)},
		{"Прогресс", fmt.Sprintf("%d%%", d.Plan.Progress)}, {"Цели", value(d.Plan.Goals)}, {},
		{"Компетенция", "Текущий уровень", "Целевой уровень"},
	}
	for _, c := range d.Plan.Competencies {
		current := "-"
		if c.CurrentLevel != nil {
			current = fmt.Sprint(*c.CurrentLevel)
		}
		rows = append(rows, []any{c.Name, current, c.TargetLevel})
	}
	rows = append(rows, []any{}, []any{"Задача", "Категория", "Приоритет", "Срок", "Статус", "Прогресс", "Оценка руководителя"})
	for _, t := range d.Tasks {
		rows = append(rows, []any{t.Title, t.Category, priorityLabel(t.Priority), t.DueDate, taskStatusLabel(t.Status), fmt.Sprintf("%d%%", t.Progress), ratingLabel(t.Rating)})
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, err
		}
	}
	header, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"2563EB"}, Pattern: 1}})
	f.SetCellStyle(sheet, "A1", "B1", header)
	f.SetCellStyle(sheet, "A9", "C9", header)
	taskHeader := 11 + len(d.Plan.Competencies)
	cell, _ := excelize.CoordinatesToCellName(7, taskHeader)
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", taskHeader), cell, header)
	f.SetColWidth(sheet, "A", "A", 34)
	f.SetColWidth(sheet, "B", "B", 24)
	f.SetColWidth(sheet, "C", "G", 20)
	var out bytes.Buffer
	if err := f.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (d *ExportDocument) PDF() ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.AddUTF8FontFromBytes("Go", "", goregular.TTF)
	p.SetMargins(16, 16, 16)
	p.SetAutoPageBreak(true, 16)
	p.AddPage()
	p.SetFont("Go", "", 18)
	p.MultiCell(0, 9, "Индивидуальный план развития", "", "L", false)
	p.SetFont("Go", "", 13)
	p.MultiCell(0, 7, d.Plan.Title, "", "L", false)
	p.Ln(2)
	p.SetFont("Go", "", 10)
	for _, line := range []string{"Сотрудник: " + d.Plan.EmployeeName, "Руководитель: " + d.Plan.ManagerName, "Период: " + d.Plan.StartDate + " - " + d.Plan.EndDate, fmt.Sprintf("Статус: %s   Прогресс: %d%%", statusLabel(d.Plan.Status), d.Plan.Progress)} {
		p.MultiCell(0, 6, line, "", "L", false)
	}
	section := func(title string) {
		p.Ln(3)
		p.SetFont("Go", "", 13)
		p.MultiCell(0, 7, title, "", "L", false)
		p.SetFont("Go", "", 10)
	}
	section("Цели")
	p.MultiCell(0, 6, value(d.Plan.Goals), "", "L", false)
	section("Компетенции")
	for _, c := range d.Plan.Competencies {
		current := "-"
		if c.CurrentLevel != nil {
			current = fmt.Sprint(*c.CurrentLevel)
		}
		p.MultiCell(0, 6, fmt.Sprintf("• %s: %s → %d", c.Name, current, c.TargetLevel), "", "L", false)
	}
	section("Задачи")
	if len(d.Tasks) == 0 {
		p.MultiCell(0, 6, "Задач нет", "", "L", false)
	}
	for i, t := range d.Tasks {
		p.SetFont("Go", "", 11)
		p.MultiCell(0, 6, fmt.Sprintf("%d. %s", i+1, t.Title), "", "L", false)
		p.SetFont("Go", "", 9)
		details := fmt.Sprintf("Статус: %s | Прогресс: %d%% | Срок: %s", taskStatusLabel(t.Status), t.Progress, fallback(t.DueDate))
		if t.Category != "" {
			details += " | Категория: " + t.Category
		}
		p.MultiCell(0, 5, details, "", "L", false)
		p.Ln(1)
	}
	var out bytes.Buffer
	if err := p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func value(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "Не указаны"
	}
	return *v
}
func fallback(v string) string {
	if v == "" {
		return "не указан"
	}
	return v
}
func statusLabel(v string) string {
	return map[string]string{"draft": "Черновик", "active": "Активен", "completed": "Завершён", "cancelled": "Отменён"}[v]
}
func taskStatusLabel(v string) string {
	return map[string]string{"not_started": "Не начата", "in_progress": "В работе", "completed": "Завершена", "cancelled": "Отменена"}[v]
}
func priorityLabel(v string) string {
	return map[string]string{"low": "Низкий", "medium": "Средний", "high": "Высокий"}[v]
}
func ratingLabel(v string) string {
	return map[string]string{"met": "Выполнено", "partially_met": "Частично выполнено", "not_met": "Не выполнено"}[v]
}
