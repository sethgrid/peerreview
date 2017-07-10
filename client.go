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

type cyclePayload struct {
	Cycles []string `json:"cycles"`
}

type goalPayload struct {
	Goal string `json:"goal"`
}

type userPayload struct {
	User UserInfo `json:"user"`
}

type revieweesPayload struct {
	Reviewees []UserInfoLite `json:"reviewees"`
}

type reviewPayload struct {
	Reviews []Review `json:"reviews"`
}

// NewClient eases test developement and potential interactions from other Go code bases
func NewClient(addr string, authkey string) *Client {
	return &Client{addr: addr, authkey: authkey}
}

// **********
// api/admin/teams
// *********

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

// **********
// api/admin/cycles
// *********

func (c *Client) GetCycles() ([]Cycle, error) {
	expectedCode := http.StatusOK
	verb := "GET"
	uri := "/api/admin/cycles"
	b, err := c.clientDo(verb, uri, expectedCode, "")
	if err != nil {
		return nil, err
	}
	var data struct {
		Cycles []Cycle `json:"cycles"`
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}
	return data.Cycles, nil
}

func (c *Client) AddCycle(cycle string) error {
	verb := "POST"
	expectedCode := http.StatusCreated
	uri := "/api/admin/cycles"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"cycle":"%s"}`, cycle))
	return err
}

func (c *Client) DeleteCycle(cycle string) error {
	verb := "DELETE"
	expectedCode := http.StatusOK
	uri := "/api/admin/cycles"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"cycle":"%s"}`, cycle))
	return err
}

func (c *Client) EditCycle(cycle string, IsOpen bool) error {
	verb := "PUT"
	expectedCode := http.StatusOK
	uri := "/api/admin/cycles"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"cycle":"%s", "is_open":%t}`, cycle, IsOpen))
	return err
}

// **********
// api/user/team
// *********

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

// **********
// /api/user
// *********

func (c *Client) GetUserInfo() (UserInfo, error) {
	var data userPayload
	expectedCode := http.StatusOK
	verb := "GET"
	uri := "/api/user"
	b, err := c.clientDo(verb, uri, expectedCode, "")
	if err != nil {
		return data.User, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return data.User, err
	}
	return data.User, nil
}

// **********
// /api/user/goal
// *********

func (c *Client) GetUsersGoal() (string, error) {
	info, err := c.GetUserInfo()
	if err != nil {
		return "", err
	}
	return info.Goals, nil
}

func (c *Client) SetUserGoal(goal string) error {
	verb := "POST"
	expectedCode := http.StatusCreated
	uri := "/api/user/goal"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"goal":"%s"}`, goal))
	return err
}

// **********
// /api/user/reviewees
// *********

func (c *Client) GetUserReviewees(cycle string) ([]UserInfoLite, error) {
	var data revieweesPayload
	expectedCode := http.StatusOK
	verb := "GET"
	uri := "/api/user/reviewees/" + cycle
	b, err := c.clientDo(verb, uri, expectedCode, "")
	if err != nil {
		return data.Reviewees, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return data.Reviewees, err
	}
	return data.Reviewees, nil
}

// **********
// /api/user/reviewer
// *********

func (c *Client) AddReviewer(email string, cycleName string) error {
	verb := "POST"
	expectedCode := http.StatusCreated
	uri := "/api/user/reviewer"
	_, err := c.clientDo(verb, uri, expectedCode, fmt.Sprintf(`{"user_email":"%s", "cycle":"%s"}`, email, cycleName))
	return err
}

// **********
// /api/user/reviews
// *********

func (c *Client) GetReviews() ([]Review, error) {
	var data reviewPayload
	expectedCode := http.StatusOK
	verb := "GET"
	uri := "/api/user/reviews"
	b, err := c.clientDo(verb, uri, expectedCode, "")
	if err != nil {
		return data.Reviews, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return data.Reviews, err
	}
	return data.Reviews, nil
}

func (c *Client) AddReviewForUser(email string, cycle string, strengths []string, opportunities []string) error {
	verb := "POST"
	expectedCode := http.StatusCreated
	uri := "/api/user/reviews"
	m := make(map[string]interface{})
	m["reviewee_email"] = email
	m["strengths"] = strengths
	m["growth_opportunities"] = opportunities
	m["cycle"] = cycle

	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	_, err = c.clientDo(verb, uri, expectedCode, string(b))
	return err
}

// **********
// helpers
// *********

func (c *Client) clientDo(verb string, uri string, expectedCode int, payload string) ([]byte, error) {
	code, b, err := c.httpDo(verb, uri, payload)
	if err != nil {
		return nil, errors.Wrapf(err, "[%d] %v", code, string(b))
	}
	if code != expectedCode {
		return nil, fmt.Errorf("got %d, want %d on %s - body: %s", code, expectedCode, uri, string(b))
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
