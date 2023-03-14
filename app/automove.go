package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

func (a Automove) Do(message_id string) error {

	var slack SlackRequest

	slack.user = User{Id: a.User.Id, TeamId: a.User.TeamId}
	slack.data = make(map[string]string)
	slack.data["channel"] = a.From
	slack.data["ts"] = message_id
	slack.data["limit"] = "30"

	thread, err := slack.GetThread()
	if thread[0].Ts != thread[0].ThreadTs && thread[0].ThreadTs != "" {
		return nil
	}
	if err != nil {
		return errors.New("Cannot retrieve thread: " + err.Error())
	}
	if !settings.NoRemove {
		for _, message := range thread {
			slack.data = make(map[string]string)
			slack.data["channel"] = a.From
			slack.data["ts"] = message.Ts
			err = slack.DeleteMessage()
			if err != nil {
				return errors.New("Cannot delete: " + message.Text + " " + err.Error())
			}
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
			fmt.Fprintln(os.Stderr, "Cannot get user: "+err.Error())
		}
		thread[i].Blocks = []Block{}

		if u.RealName != "" {
			slack.data["username"] = u.RealName
			slack.data["icon_url"] = u.Profile.Image72
		} else {
			thread[i].Blocks = append(thread[i].Blocks, Block{Type: "context", Elements: []Element{{Type: "mrkdwn", Text: "Posted by <@" + thread[i].User + ">"}}})
		}
		if len(thread[i].Text) > 0 {
			thread[i].Blocks = append(thread[i].Blocks, Block{Type: "section", Text: &Element{Type: "mrkdwn", Text: thread[i].Text}})
			slack.data["text"] += ">" + strings.ReplaceAll(thread[i].Text, "\n", "\n>")
			slack.data["text"] += "\non " + t.Format("Monday, January 2, 2006 at 15:04")
		}
		if thread[i].Attachments != nil {
			if at, err := json.Marshal(thread[i].Attachments); err == nil {
				slack.data["attachments"] = string(at)
			} else {
				fmt.Fprintln(os.Stderr, "Marshaling error: "+err.Error())
			}
		}
		if len(thread[i].Files) > 0 {
			for _, file := range thread[i].Files {
				if strings.HasPrefix(file.MimeType, "image/") {
//					thread[i].Blocks = append(thread[i].Blocks, Block{Type: "image", ImageUrl: file.PermalinkPublic, AltText: file.Title})
//					thread[i].Blocks = append(thread[i].Blocks, Block{Type: "image", ImageUrl: "https://yourbasic.org/golang/stopwatch.png", AltText: file.Title})
//				} else {
					thread[i].Blocks = append(thread[i].Blocks, Block{Type: "section", Text: &Element{Type: "mrkdwn", Text: "<" + file.UrlPrivate + "|" + file.Title + ">"}})

//				}
//				slack.data["text"] = "<" + file.UrlPrivate + "|" + file.Title + ">\n"
				}
			}
		}
		if len(thread[i].Text) > 0 {
			thread[i].Blocks = append(thread[i].Blocks, Block{Type: "context", Elements: []Element{{Type: "plain_text", Text: "on " + t.Format("Monday, January 2, 2006 at 15:04")}}})
		}

		if blocks, err := json.Marshal(thread[i].Blocks); err == nil && len(thread[i].Blocks) > 0 {
			slack.data["blocks"] = string(blocks)
		} else {
			fmt.Fprintln(os.Stderr, "Blocks list is empty or JSON error on marshal message block")
		}
		if thread[i].ThreadTs == thread[i].Ts || thread[i].Ts == "" {
			ts, err = slack.PostMessage(false)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Blocks: "+slack.data["blocks"])
				fmt.Fprintln(os.Stderr, "Cannot post: "+err.Error())
			}
			continue
		}
		slack.data["thread_ts"] = ts
		ts, err = slack.PostMessage(false)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Blocks: "+slack.data["blocks"])
			return errors.New("cannot post: " + err.Error())
		}
	}
	return nil
}
