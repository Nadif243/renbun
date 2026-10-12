package db

import (
	"database/sql"
	"fmt"
)

// SeedDefaultSources checks if the table is empty and injects the baseline domains.
func SeedDefaultSources() error {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM sources").Scan(&count)
	if err != nil {
		return err
	}

	// If URLs already exist, the user has taken control. Do not overwrite.
	if count > 0 {
		return nil
	}

	// The Built-In Defaults
	defaults := map[string][]string{
		"Science": {
			"https://ja.wikipedia.org/wiki/%E7%A7%91%E5%AD%A6",                            // Science
			"https://ja.wikipedia.org/wiki/%E7%9F%A5%E8%AD%98",                            // Knowledge
			"https://ja.wikipedia.org/wiki/%E8%87%AA%E7%84%B6",                            // Nature
			"https://ja.wikipedia.org/wiki/%E5%AE%87%E5%AE%99",                            // Universe
			"https://ja.wikipedia.org/wiki/%E8%87%AA%E7%84%B6%E7%A7%91%E5%AD%A6",          // Natural Science
			"https://ja.wikipedia.org/wiki/%E7%89%A9%E7%90%86%E5%AD%A6",                   // Physics
			"https://ja.wikipedia.org/wiki/%E5%8C%96%E5%AD%A6",                            // Chemist
			"https://ja.wikipedia.org/wiki/%E7%94%9F%E7%89%A9%E5%AD%A6",                   // Biology
			"https://ja.wikipedia.org/wiki/%E7%A7%91%E5%AD%A6%E5%8F%B2",                   // History of Science
			"https://ja.wikipedia.org/wiki/%E7%A7%91%E5%AD%A6%E7%9A%84%E6%96%B9%E6%B3%95", // Scientific Method
			"https://ja.wikipedia.org/wiki/%E7%A7%91%E5%AD%A6%E8%80%85",                   // Scientist

			"https://ja.wikipedia.org/wiki/%E7%96%91%E4%BC%BC%E7%A7%91%E5%AD%A6", // Pseudoscience
		},
		"IT": {
			// Qiita
			"https://qiita.com/carol0226/items/bdb16dbd5c16a2c43ad5",
			"https://qiita.com/shu15511551/items/ea723c5d81a14f507fba#%E4%BA%8B%E5%AE%9F%E5%AE%9F%E6%A9%9F%E7%A2%BA%E8%AA%8D%E3%83%AD%E3%82%B0%E3%81%AB%E5%9F%BA%E3%81%A5%E3%81%8F%E3%81%93%E3%81%A8",
			"https://qiita.com/shinkai_/items/4c6c12324e115a125621",
			"https://qiita.com/nogataka/items/a2f89a94d243b1b715cb",
			"https://qiita.com/shu15511551/items/fb6b9a3542ceabee2aab",
			"https://qiita.com/shu15511551/items/b4907a8a7ccead0e1c98",
			// Zenn
			"https://zenn.dev/yayo1/articles/4fb4dd2a6c4654",
			"https://zenn.dev/oyatomo/articles/9cebd37807cd4c",
			"https://zenn.dev/makihiro_dev/articles/ai-dev-assurance-over-vibe",
			"https://zenn.dev/muit_techblog/articles/6ea28887c79457",
			"https://zenn.dev/acntechjp/articles/ed556516a84934",
			"https://zenn.dev/kaityo256/articles/be_kind_to_hpc_programmers",
		},
		"Game: Genshin Lore": {
			"https://genshin-impact.fandom.com/ja/wiki/%E3%83%86%E3%82%A4%E3%83%AF%E3%83%83%E3%83%88",          // Teyvat
			"https://genshin-impact.fandom.com/ja/wiki/%E9%AD%94%E7%A5%9E%E6%88%A6%E4%BA%89",                   // Archon War
			"https://genshin-impact.fandom.com/ja/wiki/%E3%83%86%E3%82%A4%E3%83%AF%E3%83%83%E3%83%88",          // Cataclysm
			"https://genshin-impact.fandom.com/ja/wiki/%E5%A4%A9%E7%A9%BA%E3%81%AE%E5%B3%B6",                   // Celestia
			"https://genshin-impact.fandom.com/ja/wiki/%E3%82%AB%E3%83%BC%E3%83%B3%E3%83%AB%E3%82%A4%E3%82%A2", // Khaenri'ah
			"https://genshin-impact.fandom.com/ja/wiki/%E9%99%8D%E8%87%A8%E8%80%85",                            // Descender
			"https://genshin-impact.fandom.com/ja/wiki/%E9%82%AA%E7%9C%BC",                                     // Delusion
			"https://genshin-impact.fandom.com/ja/wiki/%E7%A5%9E",                                              // God
			"https://genshin-impact.fandom.com/ja/wiki/%E9%BE%8D",                                              // Dragon
			"https://genshin-impact.fandom.com/ja/wiki/%E4%B8%89%E6%9C%88",                                     // The Three Moons
			"https://genshin-impact.fandom.com/ja/wiki/%E7%A5%9E%E3%81%AE%E7%9B%AE",                            // Vision
			"https://genshin-impact.fandom.com/ja/wiki/%E7%A5%9E%E3%81%AE%E5%BF%83",                            // Gnosis
		},
		"Literature": {
			// Osamu Dazai
			"https://www.aozora.gr.jp/cards/000035/files/301_14912.html",  // No Longer Human
			"https://www.aozora.gr.jp/cards/000035/files/1565_8559.html",  // The Setting Sun
			"https://www.aozora.gr.jp/cards/000035/files/275_13903.html",  // Schoolgirl
			"https://www.aozora.gr.jp/cards/000035/files/1567_14913.html", // Run, Melos
			// Natsume Souseki
			"https://www.aozora.gr.jp/cards/000148/files/773_14560.html", // Kokoro
			"https://www.aozora.gr.jp/cards/000148/files/789_14547.html", // I Am a Cat
			"https://www.aozora.gr.jp/cards/000148/files/752_14964.html", // Botchan
			"https://www.aozora.gr.jp/cards/000148/files/799_14972.html", // Ten Nights of Dreams
			// Yoshio Toyoshima
			"https://www.aozora.gr.jp/cards/000906/files/42415_22916.html", // Out in the Open
			"https://www.aozora.gr.jp/cards/000906/files/42396_32967.html", // If I were alive
			"https://www.aozora.gr.jp/cards/000906/files/42470_31402.html", // A White Morning
			"https://www.aozora.gr.jp/cards/000906/files/42680_26265.html", // Goldenbell Flowers
		},
	}

	for category, urls := range defaults {
		for _, url := range urls {
			_, err := DB.Exec("INSERT OR IGNORE INTO sources (category, url) VALUES (?, ?)", category, url)
			if err != nil {
				return fmt.Errorf("failed to seed %s: %w", category, err)
			}
		}
	}
	return nil
}

// AddSource injects a new target URL into a category.
func AddSource(category, url string) error {
	_, err := DB.Exec("INSERT OR IGNORE INTO sources (category, url) VALUES (?, ?)", category, url)
	return err
}

// FetchUrls retrieves URLs for a specific category, or all URLs if category is "General".
func FetchUrls(category string) ([]string, error) {
	var rows *sql.Rows
	var err error

	if category == "" || category == "General" || category == "All" {
		rows, err = DB.Query("SELECT url FROM sources")
	} else {
		rows, err = DB.Query("SELECT url FROM sources WHERE category = ?", category)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

// RemoveSource deletes a URL using a 1-based index within its category.
func RemoveSource(category string, index int) error {
	rows, err := DB.Query("SELECT id FROM sources WHERE category = ? ORDER BY id ASC", category)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if index < 1 || index > len(ids) {
		return fmt.Errorf("index %d does not exist in category '%s'", index, category)
	}

	_, err = DB.Exec("DELETE FROM sources WHERE id = ?", ids[index-1])
	return err
}

// ListSources groups URLs by category for the Discord embed.
func ListSources() (map[string][]string, error) {
	rows, err := DB.Query("SELECT category, url FROM sources ORDER BY category ASC, id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string][]string)
	for rows.Next() {
		var cat, url string
		if err := rows.Scan(&cat, &url); err != nil {
			return nil, err
		}
		results[cat] = append(results[cat], url)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
