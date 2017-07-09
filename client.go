package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

type Client struct {
	addr    string
	authkey string
}

type teamPayload struct {
	Teams []string `json:"teams"`
}

// NewClient eases test developement and potential interactions from other Go code bases
func NewClient(addr string, authkey string) *Client {
	return &Client{addr: addr, authkey: authkey}
}

// InsertTeam inserts a team to be available for assignment later to a user
func (c *Client) InsertTeam(team string) error {
	verb := "POST"
	expectedCode := http.StatusCreated
	uri := "/api/admin/teams"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"team":"%s"}`, team))
	return err
}

func (c *Client) DeleteTeam(team string) error {
	verb := "DELETE"
	expectedCode := http.StatusOK
	uri := "/api/admin/teams"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"team":"%s"}`, team))
	return err
}

func (c *Client) GetTeams() ([]string, error) {
	expectedCode := http.StatusOK
	verb := "GET"
	uri := "/api/admin/teams"
	b, err := c.clientDo(verb, uri, expectedCode, "")
	if err != nil {
		return nil, err
	}
	var data teamPayload
	err = json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}
	return data.Teams, nil
}

func (c *Client) GetUsersTeams() ([]string, error) {
	expectedCode := http.StatusOK
	verb := "GET"
	uri := "/api/user/team"
	b, err := c.clientDo(verb, uri, expectedCode, "")
	if err != nil {
		return nil, err
	}
	var data teamPayload
	err = json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}
	return data.Teams, nil
}

func (c *Client) AssignTeamToUser(team string) error {
	verb := "POST"
	expectedCode := http.StatusCreated
	uri := "/api/user/team"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"team":"%s"}`, team))
	return err
}

func (c *Client) RemoveTeamFromUser(team string) error {
	verb := "DELETE"
	expectedCode := http.StatusOK
	uri := "/api/user/team"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"team":"%s"}`, team))
	return err
}

func (c *Client) clientDo(verb string, uri string, expectedCode int, payload string) ([]byte, error) {
	code, b, err := c.httpDo(verb, uri, payload)
	if err != nil {
		return nil, errors.Wrapf(err, "[%d] %v", code, string(b))
	}
	if code != expectedCode {
		return nil, fmt.Errorf("got %d, want %d on %s - %s", code, expectedCode, uri, string(b))
	}
	return b, nil
}

func (c *Client) httpDo(method string, uri string, payload string) (int, []byte, error) {
	theURL := fmt.Sprintf("%s%s", c.addr, uri)
	var resp *http.Response
	var err error

	req, err := http.NewRequest(method, theURL, strings.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Add("X-Session-Token", c.authkey)
	resp, err = http.DefaultClient.Do(req)

	if err != nil {
		return 0, nil, err
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}
