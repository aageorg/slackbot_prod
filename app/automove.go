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
	if err != nil {
		return errors.New("Cannot retrieve thread: " + err.Error())
	}
	if thread[0].Ts != thread[0].ThreadTs && thread[0].ThreadTs != "" {
		return nil
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

		if len(thread[i].Text) > 0 || len(thread[i].Files) > 0 {
			var filestring string
			if len(thread[i].Files) > 0 {
				filestring += "Uploaded file"
				if len(thread[i].Files) > 1 {
					filestring += "s"
				}
				filestring += "\n"
			}
			thread[i].Blocks = append(thread[i].Blocks, Block{Type: "context", Elements: []Element{{Type: "plain_text", Text: filestring + "on " + t.Format("Monday, January 2, 2006 at 15:04")}}})
		}
		if thread[i].Attachments != nil {
			for _, ant := range thread[i].Attachments {
				if len(ant.Files) > 0 {
					thread[i].Files = append(thread[i].Files, ant.Files...)
				}
				if len(ant.MessageBlocks) > 0 {
					if at, err := json.Marshal(thread[i].Attachments); err == nil {
						slack.data["attachments"] = string(at)
					} else {
						return errors.New("Error on marshalings attached object: " + err.Error())
					}
				}
			}
		}
		if len(thread[i].Reactions) > 0 {
			var elems []Element
			for _, r := range thread[i].Reactions {
				elems = append(elems, Element{Type: "mrkdwn", Text: ":" + r.Name + ":  *" + strconv.Itoa(r.Count) + "*"})
			}
			thread[i].Blocks = append(thread[i].Blocks, Block{Type: "context", Elements: elems})
		}

		if blocks, err := json.Marshal(thread[i].Blocks); err == nil && len(thread[i].Blocks) > 0 {
			slack.data["blocks"] = string(blocks)
		} else {
			fmt.Fprintln(os.Stderr, "Blocks list is empty or JSON error on marshal message block")
		}
		if len(thread[i].Files) > 0 {
			var filelist []map[string]string
			for _, file := range thread[i].Files {
				url, file_id, err := slack.GetUploadUrl(file.Name, file.Size)
				if err != nil {
					return errors.New("Cannot get upload url for " + file.Name + "(" + strconv.Itoa(file.Size) + "): " + err.Error())
				}
				filelist = append(filelist, map[string]string{"id": file_id, "title": file.Title})
				err = ReloadFile(file.UrlPrivate, url, file.MimeType)
				if err != nil {
					return errors.New("Cannot reload file: " + err.Error())
				}
			}
			if ts != "" && thread[i].Ts != thread[i].ThreadTs {
				slack.data["thread_ts"] = ts
			}
			m_ts, err := slack.PostMessage(false)
			if err != nil {
				return errors.New("Cannot post the first message: " + err.Error())
			}
			if ts == "" {
				ts = m_ts
			}
			err = slack.CompleteUpload(a.To, "Attached files:", ts, filelist)
			if err != nil {
				return errors.New("Cannot complete upload: " + err.Error())
			}
			for {
				msgs, err := slack.GetThreadLimit(1, a.To, ts)
				if err != nil {
					return errors.New("Cannot retrieve the last message from thread: " + err.Error())
				}
				if len(msgs) == 2 && msgs[1].Ts != m_ts {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			continue
		}
		if ts != "" {
			slack.data["thread_ts"] = ts
			_, err = slack.PostMessage(false)
		} else {
			ts, err = slack.PostMessage(false)

		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Blocks: "+slack.data["blocks"])
			return errors.New("cannot post: " + err.Error())
		}
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
	return nil
}
