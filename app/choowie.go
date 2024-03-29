package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var slackSignSecret string
var slackClientSecret string
var slackClientID string
var slackAppID string
var certFile string
var privkeyFile string

var settings Database
var voting Voting

type Voting struct {
	votes map[string]int
	mu    sync.Mutex
}

func makeVoting() Voting {
	return Voting{
		votes: make(map[string]int),
	}
}

func (v *Voting) Vote(message_ts string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.votes[message_ts]; !ok {
		v.votes[message_ts] = 0
	}
	v.votes[message_ts]++
}

func (v *Voting) UnVote(message_ts string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.votes[message_ts]; !ok {
		return fmt.Errorf("No voting for message %s was found", message_ts)
	}
	v.votes[message_ts]--
	return nil
}

func (v *Voting) Result(message_ts string) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.votes[message_ts]; ok {
		return v.votes[message_ts]
	}
	return 0
}

func (v *Voting) Cancel(message_ts string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.votes, message_ts)
}

const slackAPIUrl = "https://slack.com/api/"

var state string

func GetHash(data []byte) string {
	mac := hmac.New(sha256.New, []byte(slackSignSecret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func isVerified(headers map[string][]string, body []byte, signature string) bool {
	if len(headers["X-Slack-Request-Timestamp"]) == 0 || len(headers["X-Slack-Signature"]) == 0 {
		return false
	}
	timestamp, err := strconv.ParseInt(headers["X-Slack-Request-Timestamp"][0], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-timestamp > 60*5 {
		return false
	}

	var sig_basedata []byte
	sig_basedata = append(sig_basedata, []byte("v0:")...)
	sig_basedata = append(sig_basedata, []byte(headers["X-Slack-Request-Timestamp"][0]+":")...)
	sig_basedata = append(sig_basedata, body...)

	sign := strings.TrimPrefix(headers["X-Slack-Signature"][0], "v0=")

	if GetHash(sig_basedata) == sign {
		return true
	}
	return false
}

func OAuth(res http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	code := req.URL.Query().Get("code")
	if len(code) == 0 {
		return
	}
	var slack SlackRequest
	slack.data = make(map[string]string)
	slack.data["code"] = code
	slack.data["client_id"] = slackClientID
	slack.data["client_secret"] = slackClientSecret
	authedUsers, err := slack.OauthV2Access()
	if err != nil {
		fmt.Fprintf(res, err.Error())
		return
	}
	var output string
	for _, user := range authedUsers {
		slack.data = make(map[string]string)
		slack.data["user"] = user.Id
		slack.user.AccessToken = user.AccessToken
		if err != nil {
			fmt.Fprintf(res, err.Error())
			return
		}
		output += user.AccessToken + "\n"
	}
	fmt.Fprintf(res, output)
}

func ShowAutomoves(res http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Println("error while reading body")
		log.Fatalln(err)
	}
	if len(req.Header["X-Slack-Signature"]) == 0 || !isVerified(req.Header, body, req.Header["X-Slack-Signature"][0]) {
		res.Header().Set("Content-Type", "text/html")
		res.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(res, "403! Forbidden")
		return

	}
	q, err := url.ParseQuery(string(body))

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	command := q.Get("text")
	re := regexp.MustCompile(`\#[a-zA-Z0-9\-\_]{1,80}`)
	fromto := re.FindAllString(command, 1)

	var slack SlackRequest
	slack.user = User{Id: q.Get("user_id"), TeamId: q.Get("team_id")}
	settings.User = slack.user

	slack.data = make(map[string]string)

	slack.data["channel"] = q.Get("channel_id")
	slack.data["user"] = q.Get("user_id")
	for _, move := range settings.Automoves {
		if len(fromto) == 1 && (move.From == strings.TrimPrefix(fromto[0], "#") || move.To == strings.TrimPrefix(fromto[0], "#")) {
			slack.data["text"] += "from <#" + move.From + "> to <#" + move.To + "> on :" + move.Trigger + ":\n"
		}
		if len(fromto) == 0 {
			slack.data["text"] += "from <#" + move.From + "> to <#" + move.To + "> on :" + move.Trigger + ":\n"
		}
	}
	if len(slack.data["text"]) == 0 {
		slack.data["text"] = "No automoves found"
	} else {
		slack.data["text"] = "Automoves:\n" + slack.data["text"]
	}
	_, err = slack.PostMessage(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error on PostMessage: "+err.Error())
		return
	}
}

func CallbackHandler(res http.ResponseWriter, req *http.Request) {

	reviewReactions := func(channel, reaction, ts string) {
		if voting.Result(ts) > 0 {
			return
		}
		var slack SlackRequest
		slack.data = make(map[string]string)
		slack.data["channel"] = channel
		slack.data["latest"] = ts
		m, err := slack.RetrieveMessage()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot retrieve affected message: "+err.Error())
			return
		}
		for _, r := range m.Reactions {
			if r.Name == reaction {
				for _, u := range r.Users {
					if settings.IsPermittedUser(u) {
						voting.Vote(ts)
					}
				}
			}
		}
		voting.UnVote(ts)
	}

	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Fatalln(err)
	}
	if len(req.Header["X-Slack-Signature"]) == 0 || !isVerified(req.Header, body, req.Header["X-Slack-Signature"][0]) {
		fmt.Fprintln(os.Stderr, "Callback verification failed")
		return
	}

	var callback Callback
	err = json.Unmarshal(body, &callback)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	if callback.Type == "url_verification" {
		res.Header().Set("Content-Type", "application/json")
		resJson := "{\"challenge\":\"" + callback.Challenge + "\"}"
		fmt.Fprintf(res, resJson)
		return
	}

	if callback.Event.Type == "reaction_removed" {
		fmt.Fprintln(os.Stderr, "Event callback received: reaction "+callback.Event.Reaction+" was removed for  message "+callback.Event.Item.Ts)
		fmt.Fprintln(os.Stderr, "Necessary votes: "+strconv.Itoa(settings.NecessaryVotes)+", current votes counter: "+strconv.Itoa(voting.Result(callback.Event.Item.Ts)))
		for _, move := range settings.Automoves {
			if move.Trigger == callback.Event.Reaction && move.From == callback.Event.Item.Channel && settings.IsPermittedUser(callback.Event.User) && settings.NecessaryVotes > 0 {
				if settings.NecessaryVotes == 0 {
					return
				}
				err = voting.UnVote(callback.Event.Item.Ts)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Cannot unvote. "+err.Error())
				}
			}
		}
	}

	if callback.Event.Type == "reaction_added" {
		fmt.Fprintln(os.Stderr, "Event callback received: reaction "+callback.Event.Reaction+" on message "+callback.Event.Item.Ts)
		for _, move := range settings.Automoves {

			if move.Trigger == callback.Event.Reaction && move.From == callback.Event.Item.Channel && settings.IsPermittedUser(callback.Event.User) {
				fmt.Fprintln(os.Stderr, "Necessary votes: "+strconv.Itoa(settings.NecessaryVotes)+", current votes counter: "+strconv.Itoa(voting.Result(callback.Event.Item.Ts)))

				if settings.NecessaryVotes > 0 {
					reviewReactions(move.From, move.Trigger, callback.Event.Item.Ts)
					voting.Vote(callback.Event.Item.Ts)
					fmt.Fprintln(os.Stderr, "After previous checking, current votes counter: "+strconv.Itoa(voting.Result(callback.Event.Item.Ts)))

				}
				if voting.Result(callback.Event.Item.Ts) < settings.NecessaryVotes {
					return
				}
				fmt.Fprintln(os.Stderr, "Event ts: "+callback.Event.EventTs+": Reaction "+callback.Event.Reaction+" is trigger. Start automove.")
				voting.Cancel(callback.Event.Item.Ts)
				go func() {
					err = move.Do(callback.Event.Item.Ts)
					if err != nil {
						fmt.Fprintln(os.Stderr, err.Error())
					}
				}()
			}
		}
	}
}

func main() {

	err := settings.LoadConfig()
	if err != nil {
		panic("Cannot read a config file: " + err.Error())
	}

	slackSignSecret = settings.SlackSignSecret
	slackClientSecret = settings.SlackClientSecret
	slackClientID = settings.SlackClientId
	slackAppID = settings.SlackAppId

	http.HandleFunc("/oAuth", OAuth)
	http.HandleFunc("/showautomoves", ShowAutomoves)
	http.Handle("/setup", http.RedirectHandler("https://slack.com/oauth/v2/authorize?user_scope=chat:write&client_id="+slackClientID+"&redirect_uri="+settings.SlackBotURL+"/oAuth", http.StatusSeeOther))
	http.HandleFunc("/", CallbackHandler)
	voting = makeVoting()
	fmt.Fprintln(os.Stderr, "Slackbot started!")
	log.Fatal(http.ListenAndServe(":8080", nil))

}
