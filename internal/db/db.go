package db

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB establishes the crucible and constructs the structural geometry (tables).
func InitDB(filepath string) {
	var err error
	DB, err = sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatal("Critical: Failed to ignite the database crucible:", err)
	}
	// Enforce foreign key constraints for cascading deletes
	_, err = DB.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		log.Fatal("Critical: Failed to enforce database constraints:", err)
	}

	createTables := `
	CREATE TABLE IF NOT EXISTS users (
		discord_id TEXT PRIMARY KEY,
		bound_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS vaults (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id TEXT,
		name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (owner_id) REFERENCES users(discord_id)
	);

	CREATE TABLE IF NOT EXISTS sentences (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		vault_id INTEGER,
		keyword TEXT,
		raw_text TEXT,
		source_url TEXT,
		saved_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (vault_id) REFERENCES vaults(id)
	);`

	_, err = DB.Exec(createTables)
	if err != nil {
		log.Fatal("Critical: Failed to engrave the schema:", err)
	}

	_, err = DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_vault_text ON sentences(vault_id, raw_text);")
	if err != nil {
		log.Fatal("Critical: Failed to engrave the unique index:", err)
	}
}

// BindUser ties a Discord soul to the engine and creates their first grimoire.
func BindUser(discordID string) (string, error) {
	// Check if already bound
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE discord_id = ?)", discordID).Scan(&exists)
	if err != nil {
		return "", err
	}
	if exists {
		return "Your soul is already bound to the crucible. Use `!vault create <name>` to expand your grimoires.", nil
	}

	// Begin the binding transaction
	tx, err := DB.Begin()
	if err != nil {
		return "", err
	}

	_, err = tx.Exec("INSERT INTO users (discord_id) VALUES (?)", discordID)
	if err != nil {
		tx.Rollback()
		return "", err
	}

	_, err = tx.Exec("INSERT INTO vaults (owner_id, name) VALUES (?, 'Primary Grimoire')", discordID)
	if err != nil {
		tx.Rollback()
		return "", err
	}

	err = tx.Commit()
	if err != nil {
		return "", err
	}

	return "Binding complete. A 'Primary Grimoire' has been forged in the void.", nil
}

// CreateVault spins up a new isolated collection for the user.
func CreateVault(discordID, vaultName string) (string, error) {
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE discord_id = ?)", discordID).Scan(&exists)
	if err != nil {
		return "", err
	}
	if !exists {
		return "You are not bound to the crucible. Run `!bind` first.", nil
	}

	_, err = DB.Exec("INSERT INTO vaults (owner_id, name) VALUES (?, ?)", discordID, vaultName)
	if err != nil {
		return "", fmt.Errorf("failed to forge the vault: %w", err)
	}

	return fmt.Sprintf("Vault 『%s』 has been successfully forged.", vaultName), nil
}

// ListVaults retrieves all grimoires bound to a user.
func ListVaults(discordID string) (string, error) {
	rows, err := DB.Query("SELECT name FROM vaults WHERE owner_id = ? ORDER BY id ASC", discordID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var vaults []string
	index := 1
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", err
		}
		vaults = append(vaults, fmt.Sprintf("**%d.** 『 %s 』", index, name))
		index++
	}

	if len(vaults) == 0 {
		return "You have no forged vaults. Use `!vault create <name>`.", nil
	}

	return strings.Join(vaults, "\n"), nil
}

// OpenVault retrieves the raw Japanese text of the most recent sentences in a specific vault.
func OpenVault(discordID, vaultName string) ([]string, error) {
	query := `
		SELECT raw_text
		FROM sentences
		WHERE vault_id = (SELECT id FROM vaults WHERE owner_id = ? AND name = ?)
		ORDER BY saved_at DESC
	`
	rows, err := DB.Query(query, discordID, vaultName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sentences []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		sentences = append(sentences, text)
	}

	return sentences, nil
}

// DeleteVault incinerates a specific collection. If it has sentences, they are destroyed.
func DeleteVault(discordID, vaultName string) (string, error) {
	// Execute the deletion
	res, err := DB.Exec("DELETE FROM vaults WHERE owner_id = ? AND name = ?", discordID, vaultName)
	if err != nil {
		return "", err
	}

	// Check if the vault actually existed
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if rowsAffected == 0 {
		return "", fmt.Errorf("vault 『%s』 not found or you do not own it", vaultName)
	}

	return fmt.Sprintf("🔥 Vault 『%s』 and all its contents have been incinerated.", vaultName), nil
}

// ResolveVaultInput converts a numeric index (e.g., "1") into a vault name, or validates that a string name exists.
func ResolveVaultInput(discordID, input string) (string, error) {
	// 1. Check if input is a numeric index
	if idx, err := strconv.Atoi(input); err == nil && idx > 0 {
		var name string
		// Fetch the vault ordered by creation (id ASC) matching the index
		err := DB.QueryRow("SELECT name FROM vaults WHERE owner_id = ? ORDER BY id ASC LIMIT 1 OFFSET ?", discordID, idx-1).Scan(&name)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", fmt.Errorf("no grimoire found at index %d", idx)
			}
			return "", err
		}
		return name, nil
	}

	// 2. If it's a string, verify the vault actually exists
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM vaults WHERE owner_id = ? AND name = ?)", discordID, input).Scan(&exists)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("grimoire 『%s』 does not exist in the void", input)
	}

	return input, nil
}

// ExtractedSentence represents a raw fragment pulled from the void.
type ExtractedSentence struct {
	Keyword string
	RawText string
	Source  string
}

// SaveToVault bulk-inserts cached sentences into a specified grimoire.
func SaveToVault(discordID, vaultName string, sentences []ExtractedSentence) (int, error) {
	// 1. Get the target vault ID
	var vaultID int
	err := DB.QueryRow("SELECT id FROM vaults WHERE owner_id = ? AND name = ?", discordID, vaultName).Scan(&vaultID)
	if err != nil {
		return 0, fmt.Errorf("failed to locate vault: %w", err)
	}

	// 2. Begin transaction for bulk insert
	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO sentences (vault_id, keyword, raw_text, source_url) VALUES (?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var addedCount int64
	for _, s := range sentences {
		res, err := stmt.Exec(vaultID, s.Keyword, s.RawText, s.Source)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		aff, _ := res.RowsAffected()
		addedCount += aff
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return int(addedCount), nil
}

// DeleteSentenceFromVault removes a specific sentence from a vault using raw Japanese text matching.
func DeleteSentenceFromVault(discordID, vaultName, rawText string) error {
	var vaultID int
	err := DB.QueryRow("SELECT id FROM vaults WHERE owner_id = ? AND name = ?", discordID, vaultName).Scan(&vaultID)
	if err != nil {
		return fmt.Errorf("vault '%s' not found", vaultName)
	}

	result, err := DB.Exec("DELETE FROM sentences WHERE vault_id = ? AND raw_text = ?", vaultID, rawText)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("sentence not found in grimoire '%s'", vaultName)
	}

	return nil
}

// ClearVaultSentences purges all sentences inside a specified vault.
func ClearVaultSentences(discordID, vaultName string) (int64, error) {
	var vaultID int
	err := DB.QueryRow("SELECT id FROM vaults WHERE owner_id = ? AND name = ?", discordID, vaultName).Scan(&vaultID)
	if err != nil {
		return 0, fmt.Errorf("vault '%s' not found", vaultName)
	}

	result, err := DB.Exec("DELETE FROM sentences WHERE vault_id = ?", vaultID)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// SearchResult holds a matched sentence and its source grimoire.
type SearchResult struct {
	VaultName string
	RawText   string
}

// SearchSentences hunts for a keyword across all grimoires owned by the user.
func SearchSentences(discordID, keyword string) ([]SearchResult, error) {
	// JOIN sentences and vaults to get both the text and the vault name
	query := `
		SELECT v.name, s.raw_text
		FROM sentences s
		JOIN vaults v ON s.vault_id = v.id
		WHERE v.owner_id = ? AND s.raw_text LIKE ?
		ORDER BY s.saved_at DESC
	`
	// Wrap the keyword in wildcards for substring matching
	searchTerm := "%" + keyword + "%"

	rows, err := DB.Query(query, discordID, searchTerm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.VaultName, &res.RawText); err != nil {
			return nil, err
		}
		results = append(results, res)
	}

	return results, nil
}
