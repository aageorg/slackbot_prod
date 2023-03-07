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
	"time"
)

var slackSignSecret string
var slackClientSecret string
var slackClientID string
var slackAppID string
var certFile string
var privkeyFile string
var listenPort string

var settings Database
var waitReactionTo = make(map[string]chan string)


const DBName = "settings.db"
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
		// appuser, err := slack.GetUser()
		if err != nil {
			fmt.Fprintf(res, err.Error())
			return
		}
		output += user.AccessToken + "\n"
	}
	fmt.Fprintf(res, output)
}

func SetAutomove(res http.ResponseWriter, req *http.Request) {
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
	fromto := re.FindAllString(command, 2)
	if len(fromto) < 1 {
		res.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(res, "{\"text\": \"Usage: from #ch1 to #ch2 or just to #ch2\"}")
		return
	}
	var slack SlackRequest
	slack.data = make(map[string]string)
	slack.data["channel"] = q.Get("channel_id")
	slack.data["text"] = "Please, set automove trigger with reaction to this message"
	slack.user = User{Id: q.Get("user_id"), TeamId: q.Get("team_id")}
	message_id, err := slack.PostMessage(false)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	setNewAutomove := func(message_id string, from string, to string) {

		incoming_ch := make(chan string)
		waitReactionTo[message_id] = incoming_ch
		channel_reaction := <-incoming_ch
		// close(incoming_ch)

		re := regexp.MustCompile(`[A-Z0-9a-z\_]{3,50}`)
		chan_react := re.FindAllString(channel_reaction, 2)
		channel := chan_react[0]
		reaction := chan_react[1]

		var am Automove
		am.From = strings.TrimPrefix(from, "#")
		am.To = strings.TrimPrefix(to, "#")
		am.Trigger = reaction
		err := settings.Add(am)
		if err != nil {
			slack.data = make(map[string]string)
			slack.data["channel"] = channel
			slack.data["user"] = q.Get("user_id")
			slack.data["text"] = err.Error()
			slack.PostMessage(true)
			return
		}
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		slack.data = make(map[string]string)
		slack.data["channel"] = channel
		slack.data["ts"] = message_id
		slack.data["text"] = "Got it!"
		err = slack.UpdateMessage()

		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}
	var from, to string
	if len(fromto) == 1 {
		from = q.Get("channel_id")
		to = fromto[0]
	} else {
		from = fromto[0]
		to = fromto[1]
	}
	setNewAutomove(message_id, from, to)
}

func NoAutomove(res http.ResponseWriter, req *http.Request) {
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

	re := regexp.MustCompile(`(\#)[a-zA-Z0-9\-\_]{1,80}`)
	fromto := re.FindAllString(command, 2)
	if len(fromto) == 0 {
		res.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(res, "{\"text\": \"Usage: to #ch2\"}")
		return
	}
	var slack SlackRequest
	slack.user = User{Id: q.Get("user_id"), TeamId: q.Get("team_id")}
	settings.User = slack.user
	slack.data = make(map[string]string)

	var from, to string
	if len(fromto) == 1 {
		from = q.Get("channel_id")
		to = fromto[0]
	} else {
		from = fromto[0]
		to = fromto[1]
	}
	var am Automove
	am.From = strings.TrimPrefix(from, "#")
	am.To = strings.TrimPrefix(to, "#")
	am.User = settings.User
	err = settings.Remove(am)
	if err != nil {
		slack.data = make(map[string]string)
		slack.data["channel"] = q.Get("channel_id")
		slack.data["user"] = settings.User.Id
		slack.data["text"] = err.Error()
		slack.PostMessage(true)
		return
	}
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	slack.data["channel"] = q.Get("channel_id")
	slack.data["user"] = q.Get("user_id")

	slack.data["text"] = "Deleted automove:\nFrom <#" + am.From + "> to <#" + am.To + ">"

	_, err = slack.PostMessage(true)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
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
		fmt.Println(err.Error())
		return
	}
}

func CallbackHandler(res http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if len(req.Header["X-Slack-Signature"]) == 0 || !isVerified(req.Header, body, req.Header["X-Slack-Signature"][0]) {
		return
	}

	var callback Callback
	err = json.Unmarshal(body, &callback)
	if err != nil {
		return
	}

	if callback.Type == "url_verification" {
		res.Header().Set("Content-Type", "application/json")
		resJson := "{\"challenge\":\"" + callback.Challenge + "\"}"
		fmt.Fprintf(res, resJson)
		return
	}

	if callback.Event.Type == "reaction_added" {
		ch, ok := waitReactionTo[callback.Event.Item.Ts]
		if ok {
			delete(waitReactionTo, callback.Event.Item.Ts)
			ch <- callback.Event.Item.Channel + "|" + callback.Event.Reaction
			return
		}

		for _, move := range settings.Automoves {
			if move.Trigger == callback.Event.Reaction && move.From == callback.Event.Item.Channel {

				go move.Do(callback.Event.Item.Ts)
			}
		}
	}
}

func main() {
/*
	flag.StringVar(&slackSignSecret, "slack-sign-secret", "", "Slack signs the requests we send you using this secret")
	flag.StringVar(&slackClientSecret, "slack-client-secret", "", "Secret for making your oauth.v2.access request")
	flag.StringVar(&slackClientID, "slack-client-id", "", "Client ID for making oauth.v2.access request")
	flag.StringVar(&slackAppID, "slack-app-id", "", "App id from slack app settings")
	flag.StringVar(&listenPort, "port", "8444", "Https port where to listen incoming callbacks from Slack")

	flag.Parse()

	params := map[string]string{"slack-sign-secret": slackSignSecret,
		"slack-client-secret": slackClientSecret,
		"slack-client-id":     slackClientID,
		"slack-app-id":        slackAppID,
		"tls-cert-path":       certFile,
		"tls-privkey-path":    privkeyFile,
	}

	for param, value := range params {
		if value == "" {
			fmt.Println("--" + param + " is mandatory parameter. Use flag --help for details")
			return
		}
	}
*/

slackSignSecret = os.Getenv("SLACK_SIGN_SECRET")
slackClientSecret = os.Getenv("SLACK_CLIENT_SECRET")
slackClientID = os.Getenv("SLACK_CLIENT_ID")
slackAppID = os.Getenv("SLACK_APP_ID")
listenPort = os.Getenv("LISTEN_PORT")



//	if _, err := os.Stat(DBName); err == nil {
	settings.LoadAutomoves()
//	} else {
//		fmt.Println("Warning: You have some problem with " + DBName + " (" + err.Error() + ")")
//	}
	http.HandleFunc("/oAuth", OAuth)
	http.HandleFunc("/noautomove", NoAutomove)
	http.HandleFunc("/automove", SetAutomove)
	http.HandleFunc("/showautomoves", ShowAutomoves)
	http.Handle("/setup", http.RedirectHandler("https://slack.com/oauth/v2/authorize?scope=groups:history,users:read,commands,channels:read,channels:history,chat:write,reactions:write&user_scope=users:read,channels:read,channels:history,chat:write,reactions:write&client_id="+slackClientID+"&redirect_uri=https://choowie.appcat.cc:8444/oAuth", http.StatusSeeOther))
	http.HandleFunc("/", CallbackHandler)

	log.Fatal(http.ListenAndServe(":"+listenPort, nil))

}
