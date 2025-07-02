package database

import (
	"testing"
)

func TestDatabaseManager_ListGames(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	games, err := db.ListGames()
	if err != nil {
		t.Fatalf("Failed to list games: %v", err)
	}

	// Should have the seeded games
	if len(games) == 0 {
		t.Error("Expected games to be seeded, got none")
	}

	// Check for expected games (should have at least minecraft and cs2 based on seeding)
	gameIDs := make(map[string]bool)
	for _, game := range games {
		gameIDs[game.ID] = true
	}

	expectedGames := []string{"minecraft", "valheim", "terraria", "garrysmod", "palworld", "rust"}
	for _, expectedID := range expectedGames {
		if !gameIDs[expectedID] {
			t.Errorf("Expected game %s to be seeded", expectedID)
		}
	}
}

func TestDatabaseManager_GetGame(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test existing game
	game, err := db.GetGame("minecraft")
	if err != nil {
		t.Fatalf("Failed to get minecraft game: %v", err)
	}
	if game.ID != "minecraft" {
		t.Errorf("Expected game ID 'minecraft', got %s", game.ID)
	}
	if game.Name != "Minecraft" {
		t.Errorf("Expected game name 'Minecraft', got %s", game.Name)
	}

	// Test non-existent game
	_, err = db.GetGame("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent game")
	}
}

func TestDatabaseManager_ValidateGameID(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	tests := []struct {
		name     string
		gameID   string
		expected bool
	}{
		{"valid minecraft", "minecraft", true},
		{"valid valheim", "valheim", true},
		{"invalid game", "invalid-game", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since ValidateGameID doesn't exist, we'll check if GetGame succeeds
			_, err := db.GetGame(tt.gameID)
			result := err == nil
			if result != tt.expected {
				t.Errorf("Game %s validation = %v, expected %v", tt.gameID, result, tt.expected)
			}
		})
	}
}

func TestDatabaseManager_GamesIntegrity(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	games, err := db.ListGames()
	if err != nil {
		t.Fatalf("Failed to get games: %v", err)
	}

	// Verify each game has required fields
	for _, game := range games {
		if game.ID == "" {
			t.Error("Game has empty ID")
		}
		if game.Name == "" {
			t.Error("Game has empty Name")
		}
		if game.Image == "" {
			t.Error("Game has empty Image")
		}

		// Verify we can get each game by ID
		retrieved, err := db.GetGame(game.ID)
		if err != nil {
			t.Errorf("Failed to get game by ID %s: %v", game.ID, err)
		}
		if retrieved.ID != game.ID {
			t.Errorf("ID mismatch for game %s", game.ID)
		}
		if retrieved.Name != game.Name {
			t.Errorf("Name mismatch for game %s", game.ID)
		}
		if retrieved.Image != game.Image {
			t.Errorf("Image mismatch for game %s", game.ID)
		}
	}
}

func TestDatabaseManager_GameImageFormat(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	games, err := db.ListGames()
	if err != nil {
		t.Fatalf("Failed to get games: %v", err)
	}

	// Verify image format follows expected pattern
	expectedRegistry := "ghcr.io/0xkowalskidev/gameservers/"
	for _, game := range games {
		if game.Image == "" {
			t.Errorf("Game %s has empty image", game.ID)
			continue
		}

		// Check if image starts with expected registry
		if len(game.Image) < len(expectedRegistry) {
			t.Errorf("Game %s image '%s' is too short", game.ID, game.Image)
			continue
		}

		registryPart := game.Image[:len(expectedRegistry)]
		if registryPart != expectedRegistry {
			t.Errorf("Game %s image '%s' doesn't start with expected registry '%s'",
				game.ID, game.Image, expectedRegistry)
		}

		// Image should contain the game ID
		if game.Image[len(expectedRegistry):] == "" {
			t.Errorf("Game %s image '%s' missing game-specific part", game.ID, game.Image)
		}
	}
}
