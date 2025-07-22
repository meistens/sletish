package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sletish/internal/models"
	"strings"
)

const jikanAPIURL = "https://api.jikan.moe/v4"

func SearchAnime(query string) (*models.JikanSearchResponse, error) {
	url := fmt.Sprintf("%s/anime?q=%s&limit=5", jikanAPIURL, query)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResult models.JikanSearchResponse
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, err
	}

	return &searchResult, nil
}

func FormatAnimeMessage(animes []models.AnimeData) string {
	if len(animes) == 0 {
		return "No anime found for your search query."
	}

	var message strings.Builder
	message.WriteString("<b>Anime Search Results:</b>\n\n")

	for i, anime := range animes {
		if i >= 5 {
			break
		}

		message.WriteString(fmt.Sprintf("<b>%d. %s</b>\n", i+1, anime.Title))

		if anime.Score > 0 {
			message.WriteString(fmt.Sprintf("Score: %.1f\n", anime.Score))
		}

		if anime.Episodes > 0 {
			message.WriteString(fmt.Sprintf("Episodes: %d\n", anime.Episodes))
		}

		if anime.Year > 0 {
			message.WriteString(fmt.Sprintf("Year: %d\n", anime.Year))
		}

		if anime.Type != "" {
			message.WriteString(fmt.Sprintf("Type: %s\n", anime.Type))
		}

		if anime.Status != "" {
			message.WriteString(fmt.Sprintf("Status: %s\n", anime.Status))
		}

		if len(anime.Genres) > 0 {
			genres := make([]string, len(anime.Genres))
			for j, genre := range anime.Genres {
				genres[j] = genre.Name
			}
			message.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(genres, ", ")))
		}

		if anime.Synopsis != "" {
			synopsis := anime.Synopsis
			if len(synopsis) > 200 {
				synopsis = synopsis[:200] + "..."
			}
			message.WriteString(fmt.Sprintf("Synopsis: %s\n", synopsis))
		}

		message.WriteString(fmt.Sprintf("<a href=\"https://myanimelist.net/anime/%d\">View on MyAnimeList</a>\n", anime.MalId))
		message.WriteString("\n")
	}

	return message.String()
}
