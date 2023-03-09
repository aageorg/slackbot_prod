package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Automove struct {
	Trigger string `json:"trigger"`
	From    string `json:"from_channel"`
	To      string `json:"to_channel"`
	User    User   `json:"-"`
}

func (a Automove) Do(message_id string) {

	var slack SlackRequest

	slack.user = User{Id: a.User.Id, TeamId: a.User.TeamId}
	slack.data = make(map[string]string)
	slack.data["channel"] = a.From
	slack.data["ts"] = message_id
	slack.data["limit"] = "30"

	thread, err := slack.GetThread()
	if thread[0].Ts != thread[0].ThreadTs && thread[0].ThreadTs != "" {
		return
	}
	if err != nil {
		fmt.Println("cannot retrieve thread: " + err.Error())
		return
	}
	for _, message := range thread {
		slack.data = make(map[string]string)
		slack.data["channel"] = a.From
		slack.data["ts"] = message.Ts
		err = slack.DeleteMessage()
		if err != nil {
			fmt.Println("cannot delete: " + message.Text + " " + err.Error())
			return
		}
	}
	slack.data = make(map[string]string)
	slack.data["channel"] = a.To
	var ts string
	for i := 0; i < len(thread); i++ {
		slack.data["user"] = thread[i].User
		timestamp := strings.Split(thread[i].Ts, ".")
		unixTime, _ := strconv.ParseInt(timestamp[0], 10, 64)
		t := time.Unix(unixTime, 0)

		u, err := slack.GetUser()
		if err != nil {
			fmt.Println("cannot get user: " + err.Error())
			return
		}
		slack.data["text"] = u.RealName + " said:\n"
		if thread[i].ThreadTs == thread[i].Ts {
			slack.data["text"] += ">" + strings.ReplaceAll(thread[i].Text, "\n", "\n>")
			slack.data["text"] += "\non " + t.Format("Monday, 02 January 2006 at 15:04")
			ts, err = slack.PostMessage(false)
			if err != nil {
				fmt.Println("cannot post: " + err.Error())
				return
			}
			continue
		}
		slack.data["text"] += ">" + strings.ReplaceAll(thread[i].Text, "\n", "\n>")
		slack.data["text"] += "\non " + t.Format("Monday, 02 January 2006 at 15:04")
		slack.data["thread_ts"] = ts
		ts, err = slack.PostMessage(false)
		if err != nil {
			fmt.Println("cannot post: " + err.Error())
			return
		}
	}
}
