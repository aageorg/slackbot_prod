package main

import (
	//	"boltdb/bolt"
	"io/ioutil"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"os"
//	"time"
)

type Database struct {
	mu        sync.Mutex `json:"-"`
	User      User       `json:"-"`
	Automoves []Automove `json:"automoves"`
}

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
			if db.Automoves[i].User.GrantedThan(db.User.Id) == false {
				db.Automoves = append(db.Automoves[:i], db.Automoves[i+1:]...)

			} else {
				return errors.New("Seems that you can not change this setting")
			}
		}
	}
	if counter == 0 {
		return errors.New("No automove from <#" + a.From + "> to <#" + a.To + "> found")

	}
	db.saveAutomoves()
	return nil
}

func (db *Database) saveAutomoves() error {
	cfg, err := json.Marshal(db)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile("settings.json", cfg, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (db *Database) LoadAutomoves() error {
	cfg, err := ioutil.ReadFile("settings.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(cfg, db)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) getBotToken() string {

	return os.Getenv("SLACK_BOT_TOKEN")

}

func (db *Database) getUserToken() string {

	return os.Getenv("SLACK_USER_TOKEN")

}

/*
func (db *Database) SaveToken() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	d, err := bolt.Open(DBName, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()

	if len(db.User.AccessToken) != 0 {
		err = d.Update(func(tx *bolt.Tx) error {
			teams, err := tx.CreateBucketIfNotExists([]byte(db.User.TeamId))
			if err != nil {
				return err
			}
			cred, err := teams.CreateBucketIfNotExists([]byte("credentials"))
			if err != nil {
				return err
			}

			if db.User.IsBot == true && db.User.Profile.ApiAppId == slackAppID {
				users, err := cred.CreateBucketIfNotExists([]byte("bot"))
				if err != nil {
					return err
				}
				err = users.Put([]byte(db.User.Id), []byte(db.User.AccessToken))
				if err != nil {
					return err
				}

			}

			if db.User.IsOwner {

				users, err := cred.CreateBucketIfNotExists([]byte("owners"))
				if err != nil {
					return err
				}
				err = users.Put([]byte(db.User.Id), []byte(db.User.AccessToken))
				if err != nil {
					return err
				}

			}
			if db.User.IsAdmin {

				users, err := cred.CreateBucketIfNotExists([]byte("admins"))
				if err != nil {
					return err
				}
				err = users.Put([]byte(db.User.Id), []byte(db.User.AccessToken))
				if err != nil {
					return err
				}

			} else {
				users, err := cred.CreateBucketIfNotExists([]byte("users"))
				if err != nil {
					return err
				}
				err = users.Put([]byte(db.User.Id), []byte(db.User.AccessToken))
				if err != nil {
					return err
				}

			}

			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil

}
*/

/*
func (db *Database) saveAutomoves() error {

	d, err := bolt.Open(DBName, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()
	err = d.Update(func(tx *bolt.Tx) error {
		teams, err := tx.CreateBucketIfNotExists([]byte(db.User.TeamId))
		if err != nil {
			return err
		}
		moves := teams.Bucket([]byte("automoves"))
		if moves != nil {
			err = teams.DeleteBucket([]byte("automoves"))
			if err != nil {
				return err
			}
		}
		if len(db.Automoves) != 0 {

			moves, err = teams.CreateBucketIfNotExists([]byte("automoves"))
			if err != nil {
				return err
			}

			for _, move := range db.Automoves {

				triggers, err := moves.CreateBucketIfNotExists([]byte(move.Trigger))
				if err != nil {
					return err
				}
				froms, err := triggers.CreateBucketIfNotExists([]byte(move.From))
				if err != nil {
					return err
				}
				err = froms.Put([]byte(move.To), []byte(move.User.Id))
				if err != nil {
					return err
				}

			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil

}



func (db *Database) LoadAutomoves() error {
	d, err := bolt.Open(DBName, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	db.Automoves = []Automove{}
	d.View(func(tx *bolt.Tx) error {
		rc := tx.Cursor()
		for team, _ := rc.First(); team != nil; team, _ = rc.Next() {
			teams := tx.Bucket(team)
			if teams == nil {
				continue
			}
			moves := teams.Bucket([]byte("automoves"))
			if moves == nil {
				continue
			}
			mc := moves.Cursor()
			for trigger, _ := mc.First(); trigger != nil; trigger, _ = mc.Next() {
				triggers := moves.Bucket(trigger)
				if triggers == nil {
					continue
				}
				tc := triggers.Cursor()
				for from, _ := tc.First(); from != nil; from, _ = tc.Next() {
					froms := triggers.Bucket(from)
					if froms == nil {
						continue
					}
					fc := froms.Cursor()

					for to, userid := fc.First(); to != nil; to, userid = fc.Next() {

						var move Automove
						move.From = string(from)
						move.To = string(to)
						move.Trigger = string(trigger)
						move.User = User{Id: string(userid), TeamId: string(team)}
						db.Automoves = append(db.Automoves, move)
					}
				}
			}
		}
		return nil
	})
	return nil
}

func (db *Database) getUserToken() string {
	d, err := bolt.Open(DBName, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	var u []byte
	d.View(func(tx *bolt.Tx) error {
		team := tx.Bucket([]byte(db.User.TeamId))
		if team != nil {
			cred := team.Bucket([]byte("credentials"))
			if cred != nil {
				buckets := [3]string{"owners", "admins", "users"}
				for i := 0; i < len(buckets); i++ {

					b := cred.Bucket([]byte(buckets[i])) //try to get the most granted user.
					if b == nil {
						continue
					}
					c := b.Cursor()
					k, v := c.First()
					if k != nil {
						u = v
						break
					}

				}
			}
		}
		return nil
	})
	if u == nil {
		return ""
	}
	return string(u)

}

func (db *Database) getBotToken() string {
	d, err := bolt.Open(DBName, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	var t []byte
	d.View(func(tx *bolt.Tx) error {
		team := tx.Bucket([]byte(db.User.TeamId))
		if team != nil {
			cred := team.Bucket([]byte("credentials"))
			if cred != nil {

				b := cred.Bucket([]byte("bot"))
				if b != nil {
					c := b.Cursor()
					k, v := c.First()
					if k != nil {
						t = v
					}
				}
			}
		}
		return nil
	})
	if t == nil {
		return ""
	}
	return string(t)

}
*/
