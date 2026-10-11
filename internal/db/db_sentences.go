package db

import "fmt"

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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sentences, nil
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
