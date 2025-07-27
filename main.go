package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
)

type Schedule struct {
	StatcrewID string `json:"statcrewID"`
	HomeTeam   string `json:"homename"`
	AwayTeam   string `json:"awayname"`
	Date       string `json:"date"`
	Time       string `json:"time"`
	GameWeek   int    `json:"gameweek"`
	Location   string `json:"Location"`
	HomeScore  int    `json:"homeScore"`
	AwayScore  int    `json:"awayScore"`
	Slug       string `json:"slug"`
	GameDate   string `json:"gamedate"`
}

type Scoreboard struct {
	StatcrewID string `json:"statcrewID"`
	HomeScore  string `json:"homeScore"`
	AwayScore  string `json:"awayScore"`
	HomeRecord string `json:"homeRecord"`
	AwayRecord string `json:"awayRecord"`
}

var db *sql.DB

func main() {
	// Initialize database
	initDB()

	// Start background job to fetch data
	startDataFetcher()

	// Setup Gin router
	r := gin.Default()

	// Serve static files (for HTMX frontend)
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// API routes
	api := r.Group("/api")
	{
		api.GET("/schedule", getSchedule)
		api.GET("/scoreboard", getScoreboard)
		api.GET("/refresh", refreshData)
		api.GET("/mock", insertMockDataHandler)
	}

	// Frontend routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "European League Football",
		})
	})

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(r.Run(":8080"))
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./football.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create tables
	createTables()
}

func createTables() {
	scheduleTable := `
	CREATE TABLE IF NOT EXISTS schedule (
		statcrew_id TEXT PRIMARY KEY,
		home_team TEXT,
		away_team TEXT,
		date TEXT,
		time TEXT,
		game_week INTEGER,
		location TEXT,
		home_score INTEGER,
		away_score INTEGER,
		slug TEXT,
		game_date TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	scoreboardTable := `
	CREATE TABLE IF NOT EXISTS scoreboard (
		statcrew_id TEXT PRIMARY KEY,
		home_score TEXT,
		away_score TEXT,
		home_record TEXT,
		away_record TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(scheduleTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(scoreboardTable)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Database tables created successfully")
}

func startDataFetcher() {
	c := cron.New()

	// Fetch data every 5 minutes
	c.AddFunc("*/5 * * * *", func() {
		log.Println("Fetching new data...")
		fetchSchedule()
		// No longer need to fetch scoreboard since we calculate it from schedule
	})

	c.Start()

	// Initial fetch with fallback to mock data
	go func() {
		time.Sleep(2 * time.Second)
		fetchSchedule()

		// If no schedule data was fetched, insert mock data
		var scheduleCount int
		err := db.QueryRow("SELECT COUNT(*) FROM schedule").Scan(&scheduleCount)

		if err == nil && scheduleCount == 0 {
			log.Println("No data fetched from APIs, inserting mock data...")
			insertMockData()
		}
	}()
}

func fetchSchedule() {
	// Create a new request with the required Referer header
	req, err := http.NewRequest("GET", "https://europeanleague.football/api/schedule", nil)
	if err != nil {
		log.Printf("Error creating schedule request: %v", err)
		return
	}

	// Add the required Referer header
	req.Header.Set("Referer", "https://europeanleague.football/games/schedule")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching schedule: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Schedule API HTTP status: %d", resp.StatusCode)

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("Schedule API returned HTTP %d - API may be temporarily unavailable", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading schedule response: %v", err)
		return
	}

	// Check if response is empty or invalid
	if len(body) == 0 {
		log.Printf("Schedule API returned empty response")
		return
	}

	// Log the first 500 characters of the response for debugging
	if len(body) > 500 {
		log.Printf("Schedule API response (first 500 chars): %s", string(body[:500]))
	} else {
		log.Printf("Schedule API response: %s", string(body))
	}

	var schedules []Schedule
	if err := json.Unmarshal(body, &schedules); err != nil {
		log.Printf("Error parsing schedule JSON: %v", err)
		log.Printf("Response body: %s", string(body))
		return
	}

	// Clear existing data and insert new
	_, err = db.Exec("DELETE FROM schedule")
	if err != nil {
		log.Printf("Error clearing schedule: %v", err)
		return
	}

	if len(schedules) > 0 {
		stmt, err := db.Prepare("REPLACE INTO schedule (statcrew_id, home_team, away_team, date, time, game_week, location, home_score, away_score, slug, game_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			log.Printf("Error preparing schedule statement: %v", err)
			return
		}
		defer stmt.Close()

		for _, schedule := range schedules {
			_, err = stmt.Exec(schedule.StatcrewID, schedule.HomeTeam, schedule.AwayTeam, schedule.Date, schedule.Time, schedule.GameWeek, schedule.Location, schedule.HomeScore, schedule.AwayScore, schedule.Slug, schedule.GameDate)
			if err != nil {
				log.Printf("Error inserting schedule: %v", err)
			}
		}
	}

	log.Printf("Fetched %d schedule entries", len(schedules))
}

func fetchScoreboard() {
	resp, err := http.Get("https://europeanleague.football/api/scoreboard")
	if err != nil {
		log.Printf("Error fetching scoreboard: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Scoreboard API HTTP status: %d", resp.StatusCode)

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("Scoreboard API returned HTTP %d - API may be temporarily unavailable", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading scoreboard response: %v", err)
		return
	}

	// Check if response is empty or invalid
	if len(body) == 0 {
		log.Printf("Scoreboard API returned empty response")
		return
	}

	// Log the first 200 characters of the response for debugging
	if len(body) > 200 {
		log.Printf("Scoreboard API response (first 200 chars): %s", string(body[:200]))
	} else {
		log.Printf("Scoreboard API response: %s", string(body))
	}

	var scoreboards []Scoreboard
	if err := json.Unmarshal(body, &scoreboards); err != nil {
		log.Printf("Error parsing scoreboard JSON: %v", err)
		log.Printf("Response body: %s", string(body))
		return
	}

	// Clear existing data and insert new
	_, err = db.Exec("DELETE FROM scoreboard")
	if err != nil {
		log.Printf("Error clearing scoreboard: %v", err)
		return
	}

	if len(scoreboards) > 0 {
		stmt, err := db.Prepare("REPLACE INTO scoreboard (statcrew_id, home_score, away_score, home_record, away_record) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			log.Printf("Error preparing scoreboard statement: %v", err)
			return
		}
		defer stmt.Close()

		for _, scoreboard := range scoreboards {
			_, err = stmt.Exec(scoreboard.StatcrewID, scoreboard.HomeScore, scoreboard.AwayScore, scoreboard.HomeRecord, scoreboard.AwayRecord)
			if err != nil {
				log.Printf("Error inserting scoreboard: %v", err)
			}
		}
	}

	log.Printf("Fetched %d scoreboard entries", len(scoreboards))
}

func getSchedule(c *gin.Context) {
	rows, err := db.Query("SELECT statcrew_id, home_team, away_team, date, time, game_week, location, home_score, away_score, slug, game_date FROM schedule ORDER BY date, time")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		err := rows.Scan(&s.StatcrewID, &s.HomeTeam, &s.AwayTeam, &s.Date, &s.Time, &s.GameWeek, &s.Location, &s.HomeScore, &s.AwayScore, &s.Slug, &s.GameDate)
		if err != nil {
			log.Printf("Error scanning schedule: %v", err)
			continue
		}
		schedules = append(schedules, s)
	}

	// Check if request is from HTMX (has HX-Request header)
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "schedule.html", schedules)
	} else {
		c.JSON(http.StatusOK, schedules)
	}
}

type TeamStanding struct {
	TeamName string
	Division string
	Wins     int
	Losses   int
	Record   string
	Position int
	SoS      float64 // Strength of Schedule
	SoV      float64 // Strength of Victory
}

// Division mapping
var teamDivisions = map[string]string{
	"Vienna Vikings":       "EAST",
	"Prague Lions":         "EAST",
	"Wroclaw Panthers":     "EAST",
	"Fehérvár Enthroners":  "EAST",
	"Fehervar Enthroners":  "EAST", // Alternative spelling without accent
	"Stuttgart Surge":      "WEST",
	"Paris Musketeers":     "WEST",
	"Frankfurt Galaxy":     "WEST",
	"Cologne Centurions":   "WEST",
	"Nordic Storm":         "NORTH",
	"Rhein Fire":           "NORTH",
	"Berlin Thunder":       "NORTH",
	"Hamburg Sea Devils":   "NORTH",
	"Munich Ravens":        "SOUTH",
	"Madrid Bravos":        "SOUTH",
	"Raiders Tirol":        "SOUTH",
	"Helvetic Mercenaries": "SOUTH",
}

func getScoreboard(c *gin.Context) {
	// Get all schedule data to calculate standings
	rows, err := db.Query("SELECT home_team, away_team, home_score, away_score FROM schedule WHERE home_score > 0 OR away_score > 0 ORDER BY date, time")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Store all games for SoS/SoV calculations
	var games []struct {
		homeTeam  string
		awayTeam  string
		homeScore int
		awayScore int
	}

	// Calculate standings from schedule data
	teamStats := make(map[string]struct {
		wins   int
		losses int
	})

	for rows.Next() {
		var homeTeam, awayTeam string
		var homeScore, awayScore int
		err := rows.Scan(&homeTeam, &awayTeam, &homeScore, &awayScore)
		if err != nil {
			log.Printf("Error scanning schedule: %v", err)
			continue
		}

		// Only count games that have been played (scores > 0)
		if homeScore > 0 || awayScore > 0 {
			// Store game for SoS/SoV calculations
			games = append(games, struct {
				homeTeam  string
				awayTeam  string
				homeScore int
				awayScore int
			}{homeTeam, awayTeam, homeScore, awayScore})

			if homeScore > awayScore {
				teamStats[homeTeam] = struct {
					wins   int
					losses int
				}{teamStats[homeTeam].wins + 1, teamStats[homeTeam].losses}
				teamStats[awayTeam] = struct {
					wins   int
					losses int
				}{teamStats[awayTeam].wins, teamStats[awayTeam].losses + 1}
			} else if awayScore > homeScore {
				teamStats[awayTeam] = struct {
					wins   int
					losses int
				}{teamStats[awayTeam].wins + 1, teamStats[awayTeam].losses}
				teamStats[homeTeam] = struct {
					wins   int
					losses int
				}{teamStats[homeTeam].wins, teamStats[homeTeam].losses + 1}
			}
		}
	}

	// Calculate SoS and SoV for each team
	teamSoS := make(map[string]float64)
	teamSoV := make(map[string]float64)

	for teamName := range teamStats {
		// Calculate SoS (Strength of Schedule)
		opponentWins := 0
		opponentLosses := 0

		// Calculate SoV (Strength of Victory)
		defeatedOpponentWins := 0
		defeatedOpponentLosses := 0

		for _, game := range games {
			// Check if this team played in this game
			if game.homeTeam == teamName {
				// Team was home team
				opponent := game.awayTeam
				opponentWins += teamStats[opponent].wins
				opponentLosses += teamStats[opponent].losses

				// If team won, add opponent stats to SoV
				if game.homeScore > game.awayScore {
					defeatedOpponentWins += teamStats[opponent].wins
					defeatedOpponentLosses += teamStats[opponent].losses
				}
			} else if game.awayTeam == teamName {
				// Team was away team
				opponent := game.homeTeam
				opponentWins += teamStats[opponent].wins
				opponentLosses += teamStats[opponent].losses

				// If team won, add opponent stats to SoV
				if game.awayScore > game.homeScore {
					defeatedOpponentWins += teamStats[opponent].wins
					defeatedOpponentLosses += teamStats[opponent].losses
				}
			}
		}

		// Calculate SoS
		totalOpponentGames := opponentWins + opponentLosses
		if totalOpponentGames > 0 {
			teamSoS[teamName] = float64(opponentWins) / float64(totalOpponentGames)
		} else {
			teamSoS[teamName] = 0.0
		}

		// Calculate SoV
		totalDefeatedOpponentGames := defeatedOpponentWins + defeatedOpponentLosses
		if totalDefeatedOpponentGames > 0 {
			teamSoV[teamName] = float64(defeatedOpponentWins) / float64(totalDefeatedOpponentGames)
		} else {
			teamSoV[teamName] = 0.0
		}
	}

	// Convert to standings slice and organize by divisions
	divisionStandings := make(map[string][]TeamStanding)

	for teamName, stats := range teamStats {
		division := teamDivisions[teamName]
		if division == "" {
			division = "UNKNOWN" // Fallback for any unmapped teams
		}

		standing := TeamStanding{
			TeamName: teamName,
			Division: division,
			Wins:     stats.wins,
			Losses:   stats.losses,
			Record:   fmt.Sprintf("%d-%d", stats.wins, stats.losses),
			SoS:      teamSoS[teamName],
			SoV:      teamSoV[teamName],
		}

		divisionStandings[division] = append(divisionStandings[division], standing)
	}

	// Sort each division by wins (descending), then by losses (ascending)
	for division := range divisionStandings {
		sort.Slice(divisionStandings[division], func(i, j int) bool {
			if divisionStandings[division][i].Wins != divisionStandings[division][j].Wins {
				return divisionStandings[division][i].Wins > divisionStandings[division][j].Wins
			}
			return divisionStandings[division][i].Losses < divisionStandings[division][j].Losses
		})

		// Add position numbers within each division
		for i := range divisionStandings[division] {
			divisionStandings[division][i].Position = i + 1
		}
	}

	// Create final standings structure
	type DivisionData struct {
		Division string
		Teams    []TeamStanding
	}

	var standings []DivisionData
	divisions := []string{"EAST", "WEST", "NORTH", "SOUTH"}

	for _, division := range divisions {
		if teams, exists := divisionStandings[division]; exists {
			standings = append(standings, DivisionData{
				Division: division,
				Teams:    teams,
			})
		}
	}

	// Check if request is from HTMX (has HX-Request header)
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "scoreboard.html", standings)
	} else {
		c.JSON(http.StatusOK, standings)
	}
}

func getTeamName(statcrewID string) string {
	teamNames := map[string]string{
		"fevv2511": "Vienna Vikings",
		"pmcc2511": "Cologne Centurions",
		"mbrt2511": "Madrid Bravos",
		"hgpl2511": "Helvetic Mercenaries",
		"fgno2511": "Frankfurt Galaxy",
		"mrpw2511": "Munich Ravens",
		"sshd2511": "Stuttgart Surge",
		"rfbt2511": "Rhein Fire",
	}

	if name, exists := teamNames[statcrewID]; exists {
		return name
	}
	return statcrewID
}

func refreshData(c *gin.Context) {
	go func() {
		fetchSchedule()
		// Scoreboard is calculated from schedule data
	}()

	// Check if request is from HTMX (has HX-Request header)
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "refresh.html", gin.H{"message": "Data refresh initiated"})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Data refresh initiated"})
	}
}

func insertMockData() {
	// Insert mock schedule data
	scheduleStmt, err := db.Prepare("REPLACE INTO schedule (statcrew_id, home_team, away_team, date, time, game_week, location, home_score, away_score, slug, game_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Printf("Error preparing mock schedule statement: %v", err)
		return
	}
	defer scheduleStmt.Close()

	mockSchedules := []Schedule{
		{StatcrewID: "mock1", HomeTeam: "Manchester United", AwayTeam: "Liverpool", Date: "2024-01-15", Time: "20:00", GameWeek: 1, Location: "Manchester", HomeScore: 0, AwayScore: 0, Slug: "mock1", GameDate: "2024-01-15T20:00:00"},
		{StatcrewID: "mock2", HomeTeam: "Barcelona", AwayTeam: "Real Madrid", Date: "2024-01-16", Time: "21:00", GameWeek: 1, Location: "Barcelona", HomeScore: 0, AwayScore: 0, Slug: "mock2", GameDate: "2024-01-16T21:00:00"},
		{StatcrewID: "mock3", HomeTeam: "Bayern Munich", AwayTeam: "Borussia Dortmund", Date: "2024-01-17", Time: "19:30", GameWeek: 2, Location: "Munich", HomeScore: 0, AwayScore: 0, Slug: "mock3", GameDate: "2024-01-17T19:30:00"},
		{StatcrewID: "mock4", HomeTeam: "PSG", AwayTeam: "Marseille", Date: "2024-01-18", Time: "20:45", GameWeek: 2, Location: "Paris", HomeScore: 0, AwayScore: 0, Slug: "mock4", GameDate: "2024-01-18T20:45:00"},
	}

	for _, schedule := range mockSchedules {
		_, err = scheduleStmt.Exec(schedule.StatcrewID, schedule.HomeTeam, schedule.AwayTeam, schedule.Date, schedule.Time, schedule.GameWeek, schedule.Location, schedule.HomeScore, schedule.AwayScore, schedule.Slug, schedule.GameDate)
		if err != nil {
			log.Printf("Error inserting mock schedule: %v", err)
		}
	}

	// Insert mock scoreboard data
	scoreboardStmt, err := db.Prepare("REPLACE INTO scoreboard (statcrew_id, home_score, away_score, home_record, away_record) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Printf("Error preparing mock scoreboard statement: %v", err)
		return
	}
	defer scoreboardStmt.Close()

	mockScoreboards := []Scoreboard{
		{StatcrewID: "mock1", HomeScore: "2", AwayScore: "1", HomeRecord: "5-2", AwayRecord: "3-4"},
		{StatcrewID: "mock2", HomeScore: "0", AwayScore: "0", HomeRecord: "4-3", AwayRecord: "6-1"},
		{StatcrewID: "mock3", HomeScore: "3", AwayScore: "2", HomeRecord: "7-0", AwayRecord: "2-5"},
		{StatcrewID: "mock4", HomeScore: "1", AwayScore: "1", HomeRecord: "3-4", AwayRecord: "4-3"},
	}

	for _, scoreboard := range mockScoreboards {
		_, err = scoreboardStmt.Exec(scoreboard.StatcrewID, scoreboard.HomeScore, scoreboard.AwayScore, scoreboard.HomeRecord, scoreboard.AwayRecord)
		if err != nil {
			log.Printf("Error inserting mock scoreboard: %v", err)
		}
	}

	log.Println("Mock data inserted successfully")
}

func insertMockDataHandler(c *gin.Context) {
	// Clear existing data first
	db.Exec("DELETE FROM schedule")
	db.Exec("DELETE FROM scoreboard")

	insertMockData()

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "refresh.html", gin.H{"message": "Mock data inserted successfully"})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Mock data inserted successfully"})
	}
}
