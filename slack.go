package main

import (
	"boltdb/bolt"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type SlackRequest struct {
	method      string
	reqmethod   string
	user        User
	auth        bool
	token       string
	data        map[string]string
	contentType string
}

type Item struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
}

type Event struct {
	Type     string `json:"type"`
	Reaction string `json:"reaction"`
	EventTs  string `json:"event_ts"`
	User     string `json:"user"`
	ItemUser string `json:"item_user"`
	Ts       string `json:"ts"`
	Item     Item   `json:"item"`
}

type Team struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Callback struct {
	Token        string `json:"token"`
	TeamId       string `json:"team_id"`
	ApiAppId     string `json:"api_app_id"`
	Event        Event  `json:"event"`
	Type         string `json:"type"`
	EventContext string `json:"event_context"`
	EventId      string `json:"event_id"`
	EventTime    int64  `json:"event_time"`
	Challenge    string `json:"challenge"`
}

type Profile struct {
	ApiAppId string `json:"api_app_id"`
}

type User struct {
	Id          string  `json:"id"`
	TeamId      string  `json:"team_id"`
	UserName    string  `json:"name,omitempty"`
	RealName    string  `json:"real_name"`
	Profile     Profile `json:"profile"`
	Scope       string  `json:"scope"`
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	IsAdmin     bool    `json:"is_admin"`
	IsOwner     bool    `json:"is_owner"`
	IsBot       bool    `json:"is_bot"`
}

type Metadata struct {
	NextCursor string `json:"next_cursor"`
}

type Message struct {
	Ts       string `json:"ts"`
	ThreadTs string `json:"thread_ts"`
	User     string `json:"user"`
	Text     string `json:"text"`
}
type Response struct {
	Ok          bool      `json:"ok"`
	Error       string    `json:"error"`
	Timestamp   string    `json:"ts"`
	User        User      `json:"user"`
	AccessToken string    `json:"access_token"`
	AuthedUser  User      `json:"authed_user"`
	TokenType   string    `json:"token_type"`
	Team        Team      `json:"team"`
	Scope       string    `json:"scope"`
	BotUserId   string    `json:"bot_user_id"`
	AppId       string    `json:"app_id"`
	Messages    []Message `json:"messages"`
	Metadata    Metadata  `json:"response_metadata"`
}

func (sl SlackRequest) call() (*Response, error) {
	client := &http.Client{}
	data := url.Values{}
	for p, v := range sl.data {
		data[p] = []string{v}
	}

	querystring := ""
	var reqbody []byte
	encode := func(data map[string]string) string {
		qstring := ""
		for param, value := range data {
			qstring += url.QueryEscape(param) + "=" + url.QueryEscape(value) + "&"
		}
		return strings.TrimSuffix(qstring, "&")
	}
	if sl.method == "" {
		return nil, errors.New("API method not set")
	}
	if sl.reqmethod == "" {
		sl.reqmethod = "POST"
	}
	if sl.reqmethod == "GET" {
		querystring = "?" + encode(sl.data)
	}

	if sl.contentType == "" && sl.reqmethod == "POST" {
		sl.contentType = "application/x-www-form-urlencoded"
		data := url.Values{}
		for p, v := range sl.data {
			data[p] = []string{v}
		}

		reqbody = []byte(data.Encode())
	}
	if sl.contentType == "application/json" {
		var data = make(map[string]any)
		for k, v := range sl.data {
			if v == "true" || v == "false" {
				value, _ := strconv.ParseBool(v)
				data[k] = value
			} else {
				data[k] = sl.data[k]
			}
		}
		json_data, err := json.Marshal(data)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err

		}
		reqbody = json_data
	}

	req, err := http.NewRequest(sl.reqmethod, slackAPIUrl+sl.method+querystring, bytes.NewBuffer(reqbody))
	if err != nil {
		fmt.Println(err.Error())
		return nil, err

	}
	req.Header.Set("Content-Type", sl.contentType)
	if sl.auth == true {
		if sl.user.AccessToken == "" {
			var settings Database
			settings.User = sl.user
			sl.token = settings.getBotToken()
		} else {
			sl.token = sl.user.AccessToken
		}

		req.Header.Set("Authorization", "Bearer "+sl.token)
	}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	if response.Ok == false {
		return nil, errors.New(response.Error)

	}

	return &response, nil
}

func (sl SlackRequest) OauthV2Access() ([]User, error) {
	sl.method = "oauth.v2.access"
	result, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return []User{}, err
	}
	return result.RetrieveAuthedUsers(), nil

}

func (sl SlackRequest) PostMessage(ephemeral bool) (string, error) {
	if ephemeral == true {
		sl.method = "chat.postEphemeral"
	} else {
		sl.method = "chat.postMessage"

	}

	sl.contentType = "application/json"
	sl.auth = true
	response, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	return response.Timestamp, nil

}

func (sl SlackRequest) UpdateMessage() error {
	sl.method = "chat.update"
	sl.contentType = "application/json"
	sl.auth = true

	_, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil

}

func (sl SlackRequest) DeleteMessage() error {
	sl.method = "chat.delete"
	sl.contentType = "application/json"
	sl.auth = true
	var settings Database
	settings.User = sl.user
	sl.user.AccessToken = settings.getUserToken()
	sl.data["as_user"] = "true"
	_, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil

}

func (sl SlackRequest) GetUser() (User, error) {
	var user User
	sl.method = "users.info"
	sl.reqmethod = "GET"
	sl.auth = true
	response, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return user, err
	}
	return response.User, nil

}

func (sl SlackRequest) GetThread() ([]Message, error) {
	var mm []Message
	sl.method = "conversations.replies"
	sl.reqmethod = "GET"
	sl.auth = true
	response, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return mm, err
	}
	collect := func(r *Response) []Message {
		var m []Message

		for i := 1; i < len(r.Messages); i++ {
			m = append(m, r.Messages[i])

		}
		return m
	}
	mm = append(mm, collect(response)...)
	for response.Metadata.NextCursor != "" {
		sl.data["cursor"] = response.Metadata.NextCursor
		response, _ = sl.call()
		mm = append(collect(response), mm...)
	}
	mm = append([]Message{response.Messages[0]}, mm...)

	return mm, nil

}

func (sl SlackRequest) RetrieveMessage() (string, error) {
	sl.method = "conversations.history"
	sl.reqmethod = "GET"
	sl.data["limit"] = "1"
	sl.data["inclusive"] = "true"
	sl.auth = true
	response, err := sl.call()
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	return response.Messages[0].Text, nil

}

func (r Response) RetrieveAuthedUsers() []User {
	var users []User
	if len(r.AccessToken) > 0 {
		users = append(users, User{Id: r.BotUserId, AccessToken: r.AccessToken, TokenType: r.TokenType})

	}
	if len(r.AuthedUser.AccessToken) > 0 {
		users = append(users, User{Id: r.AuthedUser.Id, AccessToken: r.AuthedUser.AccessToken, TokenType: r.AuthedUser.TokenType})

	}
	return users
}

func (u User) GrantedThan(userid string) bool {

	d, err := bolt.Open(DBName, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	var index1 int
	var index2 int

	d.View(func(tx *bolt.Tx) error {
		index1 = 4
		index2 = 4
		team := tx.Bucket([]byte(u.TeamId))
		if team != nil {
			cred := team.Bucket([]byte("credentials"))
			if cred != nil {
				buckets := [3]string{"owners", "admins", "users"}
				for i := 0; i < 3; i++ {
					b := cred.Bucket([]byte(buckets[i]))
					if b != nil {
						u := b.Get([]byte(userid))
						if u != nil {
							if index1 > i {
								index1 = i
							}
						}
					}
				}
				for i := 0; i < 3; i++ {
					b := cred.Bucket([]byte(buckets[i]))
					if b != nil {
						u := b.Get([]byte(u.Id))
						if u != nil {
							if index2 > i {
								index2 = i
							}
						}
					}
				}
			}
		}
		return nil
	})
	if index1 > index2 {
		return true

	}
	return false

}
