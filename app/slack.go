package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SlackRequest struct {
	method      string
	source      string
	reqmethod   string
	user        User
	auth        bool
	token       string
	data        map[string]string
	contentType string
	file        string
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
	ApiAppId string `json:"api_app_id,omitempty"`
	Image72  string `json:"image_72,omitempty"`
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

type Element struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	Emoji    bool      `json:"emoji,omitempty"`
	ImageUrl string    `json:"image_url,omitempty"`
	AltText  string    `json:"alt_text,omitempty"`
	Elements []Element `json:"elements,omitempty"`
}

type Block struct {
	Type     string    `json:"type"`
	ImageUrl string    `json:"image_url,omitempty"`
	AltText  string    `json:"alt_text,omitempty"`
	Text     *Element  `json:"text,omitempty"`
	Fields   []Element `json:"fields,omitempty"`
	Elements []Element `json:"elements,omitempty"`
}

type File struct {
	Id                 string `json:"id"`
	Name               string `json:"name"`
	Title              string `json:"title"`
	Mode               string `json:"mode"`
	FileAccess         string `json:"file_access"`
	UrlPrivate         string `json:"url_private"`
	UrlPrivateDownload string `json:"url_private_download"`
	Permalink          string `json:"permalink"`
	PermalinkPublic    string `json:"permalink_public"`
	MimeType           string `json:"mimetype"`
	Size               int    `json:"size,omitempty"`
}

type Reaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
	Count int      `json:"count"`
}

type Attachment struct {
	Fallback      string        `json:"fallback,omitempty"`
	Color         string        `json:"color,omitempty"`
	Ptetext       string        `json:"pretext,omitempty"`
	AuthorName    string        `json:"author_name,omitempty"`
	AuthorLink    string        `json:"author_link,omitempty"`
	AuthorIcon    string        `json:"author_icon,omitempty"`
	Title         string        `json:"title,omitempty"`
	TitleLink     string        `json:"title_link,omitempty"`
	Text          string        `json:"text,omitempty"`
	Fields        []interface{} `json:"fields,omitempty"`
	ImageUrl      string        `json:"image_url,omitempty"`
	ThumbUrl      string        `json:"thumb_url,omitempty"`
	Footer        string        `json:"footer,omitempty"`
	FooterIcon    string        `json:"footer_icon,omitempty"`
	Ts            interface{}   `json:"ts,omitempty"`
	Files         []File        `json:"files,omitempty"`
	MessageBlocks []Block       `json:"message_blocks,omitempty"`
}

type Message struct {
	Ts          string       `json:"ts"`
	ThreadTs    string       `json:"thread_ts"`
	User        string       `json:"user"`
	Text        string       `json:"text"`
	Blocks      []Block      `json:"blocks,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Files       []File       `json:"files,omitempty"`
	Reactions   []Reaction   `json:"reactions"`
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
	File        File      `json:"file"`
	UploadURL   string    `json:"upload_url"`
	FileId      string    `json:"file_id"`
}

func (sl SlackRequest) callv2(query string, body []byte) (*Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(sl.reqmethod, slackAPIUrl+sl.method+"?"+query, bytes.NewBuffer(body))
	if err != nil {
		return nil, err

	}
	req.Header.Set("Content-Type", sl.contentType)
	if sl.auth == true {
		if sl.user.AccessToken == "" {
			sl.token = settings.getBotToken()
		} else {
			sl.token = sl.user.AccessToken
		}
		req.Header.Set("Authorization", "Bearer "+sl.token)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	if response.Ok == false {
		return nil, errors.New(response.Error)

	}
	return &response, nil

}

func (sl SlackRequest) call() (*Response, error) {
	if sl.method == "" {
		return nil, errors.New("API method not set")
	}

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
	if sl.reqmethod == "" {
		sl.reqmethod = "POST"
	}
	if sl.reqmethod == "GET" {
		querystring = "?" + encode(sl.data)
	}

	if sl.contentType == "" && sl.reqmethod == "POST" {
		sl.contentType = "application/x-www-form-urlencoded"
		//		data := url.Values{}
		//		for p, v := range sl.data {
		//			data[p] = []string{v}
		//		}

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
			return nil, err

		}
		reqbody = json_data
	}
	if sl.contentType == "multipart/form-data" {
		file, err := os.Open(sl.file)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", filepath.Base(sl.file))
		if err != nil {
			return nil, err
		}
		n, err := io.Copy(part, file)
		fmt.Fprintf(os.Stderr, "copied bytes: %d\n", n)
		for key, val := range sl.data {
			_ = writer.WriteField(key, val)
		}
		err = writer.Close()
		if err != nil {
			return nil, err
		}
		sl.contentType = writer.FormDataContentType()
		reqbody = body.Bytes()
	}

	req, err := http.NewRequest(sl.reqmethod, slackAPIUrl+sl.method+querystring, bytes.NewBuffer(reqbody))
	if err != nil {
		return nil, err

	}
	req.Header.Set("Content-Type", sl.contentType)
	if sl.auth == true {
		if sl.user.AccessToken == "" {
			sl.token = settings.getBotToken()
		} else {
			sl.token = sl.user.AccessToken
		}

		req.Header.Set("Authorization", "Bearer "+sl.token)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
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
		return err
	}
	return nil

}

func (sl SlackRequest) DeleteMessage() error {
	sl.method = "chat.delete"
	sl.contentType = "application/json"
	sl.auth = true
	settings.User = sl.user
	sl.user.AccessToken = settings.getUserToken()
	sl.data["as_user"] = "true"
	_, err := sl.call()
	if err != nil {
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

func (sl SlackRequest) GetThreadLimit(limit int, channel string, thread_ts string) ([]Message, error) {
	sl.method = "conversations.replies"
	sl.contentType = "application/x-www-form-urlencoded"
	sl.reqmethod = "GET"
	sl.auth = true
	v := url.Values{}
	v.Add("limit", strconv.Itoa(limit))
	v.Add("channel", channel)
	v.Add("ts", thread_ts)
	req := v.Encode()
	res, err := sl.callv2(req, nil)
	if err != nil {
		return nil, err
	}
	return res.Messages, nil
}

func (sl SlackRequest) RetrieveMessage() (Message, error) {
	sl.method = "conversations.history"
	sl.reqmethod = "GET"
	sl.data["limit"] = "1"
	sl.data["inclusive"] = "true"
	sl.auth = true
	res, err := sl.call()
	if err != nil {
		return Message{}, err
	}
	return res.Messages[0], nil
}

func (sl SlackRequest) FileInfo(file_id string) (File, error) {
	sl.method = "files.info"
	sl.reqmethod = "GET"
	sl.auth = true
	v := url.Values{}
	v.Add("file", file_id)
	req := v.Encode()
	res, err := sl.callv2(req, nil)
	if err != nil {
		return File{}, err
	}
	return res.File, nil
}

func (sl SlackRequest) GetUploadUrl(filename string, filesize int) (string, string, error) {
	sl.method = "files.getUploadURLExternal"
	sl.reqmethod = "GET"
	sl.contentType = "application/x-www-form-urlencoded"
	sl.auth = true
	v := url.Values{}
	v.Add("length", strconv.Itoa(filesize))
	v.Add("filename", filename)
	req := v.Encode()
	res, err := sl.callv2(req, nil)
	if err != nil {
		return "", "", err
	}
	return res.UploadURL, res.FileId, nil
}

func (sl SlackRequest) CompleteUpload(to_channel string, comment string, thread_ts string, files []map[string]string) error {
	sl.method = "files.completeUploadExternal"
	sl.contentType = "application/json"
	sl.reqmethod = "POST"
	sl.auth = true
	params := make(map[string]any)
	params["files"] = files
	params["channel_id"] = to_channel
	params["thread_ts"] = thread_ts
	params["initial_comment"] = comment
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = sl.callv2("", body)
	if err != nil {
		return err
	}
	return nil
}

func (sl SlackRequest) AttachFiles(channel string, ts string, message string, files []string) error {
	sl.method = "chat.update"
	sl.contentType = "application/json"
	sl.reqmethod = "POST"
	sl.auth = true
	params := make(map[string]any)
	params["file_ids"] = files
	params["as_user"] = true
	params["text"] = message
	params["channel"] = channel
	params["ts"] = ts
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = sl.callv2("", body)
	if err != nil {
		return err
	}
	return nil

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

func ReloadFile(url_from string, url_to string, content_type string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url_from, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+settings.getBotToken())
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	res, err = http.Post(url_to, content_type, res.Body)
	if err != nil {
		return err
	}
	return nil
}
