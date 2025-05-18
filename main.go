package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

var (
	API_KEY  string
	STEAM_ID string
)

const (
	BASE_URL  = "http://api.steampowered.com"
	STORE_URL = "https://store.steampowered.com/api/appdetails"
)

type gameResult struct {
	name        string
	supportsMac bool
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	API_KEY = os.Getenv("API_KEY")
	STEAM_ID = os.Getenv("STEAM_ID_64")

	if API_KEY == "" || STEAM_ID == "" {
		log.Fatal("API_KEY and STEAM_ID_64 must be set")
	}

	steamGames, err := getSteamGames()
	if err != nil {
		log.Fatal("Error getting Steam games")
	}

	macGames := make([]string, 0)
	nonMacGames := make([]string, 0)

	// Create channels for results
	resultChan := make(chan gameResult, len(steamGames))

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Launch goroutines for each game
	for _, game := range steamGames {
		wg.Add(1)
		go func(appID int) {
			defer wg.Done()
			supportsMac, name := checkMacOSSupport(appID)
			resultChan <- gameResult{name: name, supportsMac: supportsMac}
		}(game.AppID)
	}

	// Close the result channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		if result.supportsMac {
			macGames = append(macGames, result.name)
		} else {
			nonMacGames = append(nonMacGames, result.name)
		}
	}

	// Write macGames to mac_games.txt and nonMacGames to non_mac_games.txt
	macGamesFile, err := os.Create("mac_games.txt")
	if err != nil {
		log.Fatal("Error creating mac_games.txt", err)
	}
	defer macGamesFile.Close()

	nonMacGamesFile, err := os.Create("non_mac_games.txt")
	if err != nil {
		log.Fatal("Error creating non_mac_games.txt", err)
	}
	defer nonMacGamesFile.Close()

	for _, game := range macGames {
		if game == "" {
			continue
		}
		macGamesFile.WriteString(game + "\n")
	}

	for _, game := range nonMacGames {
		if game == "" {
			continue
		}
		nonMacGamesFile.WriteString(game + "\n")
	}
}

type SteamGamesResponse struct {
	Response struct {
		Games     []SteamGame `json:"games"`
		GameCount int         `json:"game_count"`
	} `json:"response"`
}

type SteamGame struct {
	AppID int `json:"appid"`
}

func getSteamGames() ([]SteamGame, error) {
	queryParams := map[string]string{
		"key":             API_KEY,
		"steamid":         STEAM_ID,
		"format":          "json",
		"include_appinfo": "True",
	}

	queryParamsString := url.Values{}
	for key, value := range queryParams {
		queryParamsString.Add(key, value)
	}

	url := fmt.Sprintf("%s/IPlayerService/GetOwnedGames/v0001/?%s", BASE_URL, queryParamsString.Encode())

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal("Error fetching Steam games", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading Steam games", err)
	}

	var steamGamesResponse SteamGamesResponse
	err = json.Unmarshal(body, &steamGamesResponse)
	if err != nil {
		log.Fatal("Error parsing Steam games", err)
	}

	return steamGamesResponse.Response.Games, nil
}

type StoreGameResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Name      string `json:"name"`
		Platforms struct {
			Mac bool `json:"mac"`
		} `json:"platforms"`
	} `json:"data"`
}

func checkMacOSSupport(appID int) (bool, string) {
	url := fmt.Sprintf("%s?appids=%d", STORE_URL, appID)

	resp, err := http.Get(url)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, ""
	}

	var response map[string]StoreGameResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, ""
	}

	gameResponse, ok := response[fmt.Sprintf("%d", appID)]
	if !ok {
		return false, ""
	}

	if !gameResponse.Success {
		return false, ""
	}

	supportsMac := gameResponse.Data.Platforms.Mac

	if supportsMac {
		fmt.Printf("✅ %s\n", gameResponse.Data.Name)
	} else {
		fmt.Printf("❌ %s\n", gameResponse.Data.Name)
	}

	return supportsMac, gameResponse.Data.Name
}
