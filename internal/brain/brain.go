package brain

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

type KnowledgeEntry struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Tags      string `json:"tags"`
	FileRefs  string `json:"file_refs"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

type Brain struct {
	db *sql.DB
}

func NewBrain(db *sql.DB) *Brain {
	return &Brain{db: db}
}

func (b *Brain) Save(entry *KnowledgeEntry, requireConfirmation bool) (int64, error) {
	if requireConfirmation {
		// In interactive mode, this would prompt the user
		// For now, we'll just set status to temp
		entry.Status = "temp"
	}

	if entry.Status == "" {
		entry.Status = "temp"
	}

	entry.CreatedAt = time.Now().Unix()

	result, err := b.db.Exec(
		`INSERT INTO knowledge_entries (type, title, content, tags, file_refs, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Type, entry.Title, entry.Content, entry.Tags, entry.FileRefs, entry.Status, entry.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("error inserting knowledge entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting entry ID: %w", err)
	}

	// Insert into FTS
	_, err = b.db.Exec(
		`INSERT INTO knowledge_fts (title, content, tags) VALUES (?, ?, ?)`,
		entry.Title, entry.Content, entry.Tags,
	)
	if err != nil {
		return 0, fmt.Errorf("error inserting into FTS: %w", err)
	}

	return id, nil
}

// SaveIfNotExists persiste la entrada sólo si no existe ya otra con el
// mismo título. Se usa para sembrar el brain desde la clasificación
// semántica (`androideai init`) sin generar duplicados cuando el
// usuario corre el comando varias veces.
//
// Devuelve (id, true, nil) si insertó una nueva fila,
// (existingID, false, nil) si ya existía una con ese título, o
// (0, false, err) si algo falló.
func (b *Brain) SaveIfNotExists(entry *KnowledgeEntry) (int64, bool, error) {
	if entry.Status == "" {
		entry.Status = "temp"
	}

	var existingID int64
	err := b.db.QueryRow(
		`SELECT id FROM knowledge_entries WHERE title = ? LIMIT 1`,
		entry.Title,
	).Scan(&existingID)
	if err == nil {
		return existingID, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("error checking existing entry: %w", err)
	}

	id, err := b.Save(entry, false)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (b *Brain) Search(query string) ([]*KnowledgeEntry, error) {
	rows, err := b.db.Query(
		`SELECT id, type, title, content, tags, file_refs, status, created_at
		FROM knowledge_entries
		WHERE id IN (
			SELECT rowid FROM knowledge_fts WHERE knowledge_fts MATCH ?
		)
		ORDER BY created_at DESC`,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("error searching knowledge: %w", err)
	}
	defer rows.Close()

	var entries []*KnowledgeEntry
	for rows.Next() {
		entry := &KnowledgeEntry{}
		if err := rows.Scan(&entry.ID, &entry.Type, &entry.Title, &entry.Content,
			&entry.Tags, &entry.FileRefs, &entry.Status, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (b *Brain) Review() ([]*KnowledgeEntry, error) {
	rows, err := b.db.Query(
		`SELECT id, type, title, content, tags, file_refs, status, created_at
		FROM knowledge_entries
		WHERE status = 'temp'
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("error reviewing knowledge: %w", err)
	}
	defer rows.Close()

	var entries []*KnowledgeEntry
	for rows.Next() {
		entry := &KnowledgeEntry{}
		if err := rows.Scan(&entry.ID, &entry.Type, &entry.Title, &entry.Content,
			&entry.Tags, &entry.FileRefs, &entry.Status, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (b *Brain) Promote(id int64) error {
	result, err := b.db.Exec(
		`UPDATE knowledge_entries SET status = 'promoted' WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("error promoting entry: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("entry with id %d not found", id)
	}

	return nil
}

func (b *Brain) List() ([]*KnowledgeEntry, error) {
	rows, err := b.db.Query(
		`SELECT id, type, title, content, tags, file_refs, status, created_at
		FROM knowledge_entries
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("error listing knowledge: %w", err)
	}
	defer rows.Close()

	var entries []*KnowledgeEntry
	for rows.Next() {
		entry := &KnowledgeEntry{}
		if err := rows.Scan(&entry.ID, &entry.Type, &entry.Title, &entry.Content,
			&entry.Tags, &entry.FileRefs, &entry.Status, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (b *Brain) ExportToMarkdown(filePath string) error {
	entries, err := b.List()
	if err != nil {
		return fmt.Errorf("error listing entries: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Project Knowledge\n\n")
	sb.WriteString("Exported from androideai-core brain\n\n")

	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("## %s\n\n", entry.Title))
		sb.WriteString(fmt.Sprintf("**Type:** %s  \n", entry.Type))
		sb.WriteString(fmt.Sprintf("**Status:** %s  \n", entry.Status))
		sb.WriteString(fmt.Sprintf("**Created:** %s  \n\n", time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05")))

		if entry.Tags != "" {
			sb.WriteString(fmt.Sprintf("**Tags:** %s  \n\n", entry.Tags))
		}

		if entry.FileRefs != "" {
			sb.WriteString(fmt.Sprintf("**References:** %s  \n\n", entry.FileRefs))
		}

		sb.WriteString(fmt.Sprintf("%s\n\n", entry.Content))
		sb.WriteString("---\n\n")
	}

	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("error writing markdown file: %w", err)
	}

	return nil
}

func (b *Brain) ImportFromMarkdown(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading markdown file: %w", err)
	}

	content := string(data)
	sections := strings.Split(content, "---")

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" || strings.HasPrefix(section, "# Project Knowledge") {
			continue
		}

		// Parse section
		lines := strings.Split(section, "\n")
		if len(lines) < 2 {
			continue
		}

		// Extract title (remove ## prefix)
		title := strings.TrimPrefix(lines[0], "## ")
		title = strings.TrimSpace(title)

		// Extract metadata and content
		var entryType, status, tags, fileRefs, entryContent string
		var contentStart int

		for i, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "**Type:**") {
				entryType = strings.TrimPrefix(line, "**Type:**")
				entryType = strings.TrimSpace(entryType)
			} else if strings.HasPrefix(line, "**Status:**") {
				status = strings.TrimPrefix(line, "**Status:**")
				status = strings.TrimSpace(status)
			} else if strings.HasPrefix(line, "**Tags:**") {
				tags = strings.TrimPrefix(line, "**Tags:**")
				tags = strings.TrimSpace(tags)
			} else if strings.HasPrefix(line, "**References:**") {
				fileRefs = strings.TrimPrefix(line, "**References:**")
				fileRefs = strings.TrimSpace(fileRefs)
			} else if line != "" && !strings.HasPrefix(line, "**") {
				contentStart = i + 1
				break
			}
		}

		// Get content
		if contentStart > 0 {
			contentLines := lines[contentStart:]
			entryContent = strings.Join(contentLines, "\n")
		}

		// Create entry
		entry := &KnowledgeEntry{
			Type:     entryType,
			Title:    title,
			Content:  entryContent,
			Tags:     tags,
			FileRefs: fileRefs,
			Status:   status,
		}

		// Save entry
		if _, err := b.Save(entry, false); err != nil {
			return fmt.Errorf("error saving entry: %w", err)
		}
	}

	return nil
}
