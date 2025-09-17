package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Neon Charm-inspired color palette
var (
	primaryColor   = lipgloss.Color("#FF5FEF") // Neon pink
	secondaryColor = lipgloss.Color("#00F8D4") // Electric teal
	accentColor    = lipgloss.Color("#BD93FF") // Neon purple
	textColor      = lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#E0E0E0"}
	subtleColor    = lipgloss.Color("#A0A0A0")
	highlightColor = lipgloss.Color("#FFB86C") // Neon peach
)

// Define all our styles
var (
	// Text elements
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginRight(1)

	episodeStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	timeStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			PaddingLeft(1)

	scoreStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			PaddingLeft(1)

	noScoreStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true).
			PaddingLeft(1)

	// Day headers
	dayHeaderStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			MarginTop(1).
			Underline(true).
			PaddingBottom(0)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#444"}).
			SetString("╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌")

	// Containers
	showEntryStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0)

	appStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#BD93FF", Dark: "#BD93FF"}).
			Foreground(textColor)
)

type GraphQLRequest struct {
	Query string `json:"query"`
}

type GraphQLResponse struct {
	Data struct {
		Page struct {
			AiringSchedules []AiringSchedule `json:"airingSchedules"`
		} `json:"Page"`
	} `json:"data"`
}

type AiringSchedule struct {
	Episode   int `json:"episode"`
	AiringAt int `json:"airingAt"`
	Media     struct {
		Title struct {
			Romaji  string `json:"romaji"`
			English string `json:"english"`
		} `json:"title"`
		AverageScore int `json:"averageScore"`
	} `json:"media"`
}

type ShowInfo struct {
	Title         string
	EpisodeNumber int
	AverageScore  int
	AiringTime    time.Time
}

func main() {
	now := time.Now().UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7)

	query := fmt.Sprintf(`
	{
		Page(perPage: 100) {
			airingSchedules(airingAt_greater: %d, airingAt_lesser: %d, sort: TIME_DESC) {
				episode
				airingAt
				media {
					title {
						romaji
						english
					}
					averageScore
				}
			}
		}
	}
	`, sevenDaysAgo.Unix(), now.Unix())

	requestBody, err := json.Marshal(GraphQLRequest{Query: query})
	if err != nil {
		printError("Error creating request", err)
		return
	}

	resp, err := http.Post("https://graphql.anilist.co", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		printError("Error making request", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		printError("Error reading response", err)
		return
	}

	var response GraphQLResponse
	if err := json.Unmarshal(body, &response); err != nil {
		printError("Error parsing response", err)
		return
	}

	showsByDay := organizeShowsByDay(response.Data.Page.AiringSchedules)
	if len(showsByDay) == 0 {
		fmt.Println(appStyle.Render("✨ No new episodes aired in the past week ✨"))
		return
	}

	renderOutput(showsByDay)
}

func organizeShowsByDay(schedules []AiringSchedule) map[time.Time][]ShowInfo {
	showsByDay := make(map[time.Time][]ShowInfo)

	for _, schedule := range schedules {
		title := schedule.Media.Title.English
		if title == "" {
			title = schedule.Media.Title.Romaji
		}

		airTime := time.Unix(int64(schedule.AiringAt), 0).UTC()
		dayKey := time.Date(airTime.Year(), airTime.Month(), airTime.Day(), 0, 0, 0, 0, time.UTC)

		showsByDay[dayKey] = append(showsByDay[dayKey], ShowInfo{
			Title:         title,
			EpisodeNumber: schedule.Episode,
			AverageScore:  schedule.Media.AverageScore,
			AiringTime:    airTime,
		})
	}

	return showsByDay
}

func renderOutput(showsByDay map[time.Time][]ShowInfo) {
	var output strings.Builder

	// Get sorted days
	days := make([]time.Time, 0, len(showsByDay))
	for day := range showsByDay {
		days = append(days, day)
	}
	// Sort in reverse chronological order
	sort.Slice(days, func(i, j int) bool {
		return days[i].After(days[j])
	})

	// Build output with enhanced styling
	for i, day := range days {
		shows := showsByDay[day]
		dayFormatted := day.Format("Monday (Jan 02)")

		// Day header with subtle divider
		header := dayHeaderStyle.Render("📺 " + dayFormatted)
		output.WriteString(header + "\n")
		output.WriteString(dividerStyle.String() + "\n")

		// Shows for this day
		for _, show := range shows {
			emoji := "✨"
			if show.AverageScore > 75 {
				emoji = "🌟"
			} else if show.AverageScore == 0 {
				emoji = "📡"
			}

			entry := showEntryStyle.Render(
				emoji + " " +
					titleStyle.Render(show.Title) +
					lipgloss.NewStyle().Foreground(subtleColor).Render(" • ") +
					episodeStyle.Render(fmt.Sprintf("Ep %d", show.EpisodeNumber)) +
					timeStyle.Render(fmt.Sprintf(" 🕒 %s", show.AiringTime.Format("3:04 PM"))) +
					renderScore(show.AverageScore),
			)
			output.WriteString(entry + "\n")
		}

		// Add space between days (but not after last day)
		if i < len(days)-1 {
			output.WriteString("\n")
		}
	}

	// Final render with beautiful border
	fmt.Println(appStyle.Render(output.String()))
}

func renderScore(score int) string {
	if score > 0 {
		return scoreStyle.Render(fmt.Sprintf("★ %.0f/100", float32(score)))
	}
	return noScoreStyle.Render("★ No rating")
}

func printError(context string, err error) {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true)
	
	fmt.Println(errorStyle.Render(fmt.Sprintf("%s: %v", context, err)))
}

