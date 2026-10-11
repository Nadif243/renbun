package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

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

	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(vaults) == 0 {
		return "You have no forged vaults. Use `!vault create <name>`.", nil
	}

	return strings.Join(vaults, "\n"), nil
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
