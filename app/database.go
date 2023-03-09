package main

import (
	"encoding/json"
	"os"
	"sync"
)

type Database struct {
	mu                sync.Mutex `json:"-"`
	User              User       `json:"-"`
	SlackSignSecret   string     `json:"slack_sign_secret"`
	SlackClientSecret string     `json:"slack_client_secret"`
	SlackClientId     string     `json:"slack_client_id"`
	SlackAppId        string     `json:"slack_app_id"`
	SlackUserToken    string     `json:"slack_user_token"`
	SlackBotToken     string     `json:"slack_bot_token"`
	SlackBotURL       string     `json:"slack_bot_url"`
	Automoves         []Automove `json:"automoves"`
}

/*
func (db *Database) Add(a Automove) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if a.From == a.To {
		return errors.New("You can not create an automove from <#" + a.From + "> to itself.")
	}
	for i := 0; i < len(db.Automoves); i++ {
		if (db.Automoves[i].From == a.From && db.Automoves[i].To == a.To && db.Automoves[i].Trigger != a.Trigger) || (db.Automoves[i].From == a.From && db.Automoves[i].Trigger == a.Trigger) {
			return errors.New("You already have an automove to <#" + db.Automoves[i].To + "> on " + ":" + db.Automoves[i].Trigger + ":.")
		}
	}

	db.Automoves = append(db.Automoves, a)
	db.saveAutomoves()
	return nil
}

func (db *Database) Remove(a Automove) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	counter := 0
	for i := 0; i < len(db.Automoves); i++ {
		if db.Automoves[i].From == a.From && db.Automoves[i].To == a.To {
			counter++
			db.Automoves = append(db.Automoves[:i], db.Automoves[i+1:]...)
		}
	}
	if counter == 0 {
		return errors.New("No automove from <#" + a.From + "> to <#" + a.To + "> found")

	}
	return nil
}

func (db *Database) saveAutomoves() error {
	cfg, err := json.Marshal(db)
	if err != nil {
		return err
	}
	err = os.WriteFile("settings.json", cfg, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}
*/

func (db *Database) LoadConfig() error {
	cfg, err := os.ReadFile("config/config.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(cfg, &db)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) getBotToken() string {
	return db.SlackBotToken

}

func (db *Database) getUserToken() string {
	return db.SlackUserToken

}
