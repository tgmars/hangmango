package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

// HangmanState ... State of a game per client.
type HangmanState struct {
	// client  *Client
	turn        bool
	answer      string
	guesses     []string
	wordguesses []string
	hint        string
	valid       bool
	score       int
}

var answerPool = []string{"apple", "hello", "laminate", "sorcerer", "willow"}

// NewGame ... Initialise a game with a new random word.
func (state *HangmanState) NewGame() {
	rand.Seed(time.Now().UnixNano())
	state.answer = answerPool[rand.Intn(len(answerPool))]
	state.hint = generateStringOfLength(len(state.answer), '_')
}

// process ... Handles turn by turn logic for a hangman game and returns
// the string to send back to the client.
// Guesses over 100 characters in length return an error message back
// to the client.
func (state *HangmanState) process(message string) string {
	message = strings.ToLower(message)

	if len(message) == 1 {
		state.guesses = append(state.guesses, message)
		// single letter guess
		positions, err := getPositionsInString(state.answer, message)
		if err != nil {
			log.Printf(" - ERROR - %s", fmt.Errorf("%s", err))
		}
		state.updateHint(positions, message)
		if strings.Index(state.hint, "_") == -1 {
			// If there's no more underscores in the server generated hint string, the player has guessed the correct word.
			state.calculateScore()
			state.valid = false
			return fmt.Sprintf("%d", state.score)
		}
	}
	if (len(message) > 1) && (len(message) <= 100) {
		// word guess, only correct if the client guesses the entire answer.
		state.wordguesses = append(state.wordguesses, message)
		if state.answer == message {
			state.calculateScore()
			state.valid = false
			return fmt.Sprintf("%d", state.score)
		}
	}
	if len(message) > 100 {
		return "Guesses are limited to 100 characters in length."
	}
	return state.hint
}

// updateHint ... uses a list of integers that represent positions in the answer
// string to fill a correctly guessed character in the state.hint
func (state *HangmanState) updateHint(positions []int, updateChar string) {
	temp := []byte(state.hint)
	for _, c := range positions {
		temp[c] = []byte(updateChar)[0]
	}
	state.hint = string(temp)
}

// calculateScore ... Calulate the state's score using the formula prescribed in the criteria.
func (state *HangmanState) calculateScore() {
	state.score = 10*len(state.answer) - 2*len(state.guesses) - len(state.wordguesses)
}

// generateStringOfLength ... returns a string of the specified length,
// consisting of the provided character rune.
// generateStringOfLength(4,'A') returns string "AAAA"
func generateStringOfLength(length int, char rune) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = char
	}
	return string(b)
}

// getPositionsInString ... returns a slice of integers containing the indexes
// in message for each occurance of the search character.A maximum of 10 matches will
// be returned. For example, if the target was "AAAAA" and the search was 'A',
// a slice of [0,1,2,3,4] will be returned.
func getPositionsInString(target string, search string) ([]int, error) {
	maxlength := 10
	var indexes []int
	for i, c := range target {
		if string(c) == search {
			if len(indexes) <= maxlength {
				indexes = append(indexes, i)
			} else {
				return nil, errors.New("Hangman::getPositionsInString() - Number of runes matched in the message string is greater than the hardcoded limit, 10")
			}
		}
	}
	return indexes, nil
}
