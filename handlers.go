package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/aymerick/raymond"
)

func (a app) rootHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		log.Println("error reading auth cooking ", err)
	} else {
		if _, ok := IsValidAuth(cookie.Value); ok {
			log.Println("user is logged in")
			http.Redirect(w, r, "dash", http.StatusTemporaryRedirect)
			return
		}
	}
	b, err := ioutil.ReadFile("web/static/index.html")
	if err != nil {
		LogEntrySetField(r, "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func (a app) dashHandler(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		log.Println("something went wrong")
		return
	}
	tmpl, err := ioutil.ReadFile("web/templates/dash.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	ctx := map[string]string{
		"email": email,
	}
	result, err := raymond.Render(string(tmpl), ctx)
	w.Write([]byte(result))
}

func (a app) tokenHandler(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("idtoken")
	resp, err := http.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + token)
	if err != nil {
		log.Println(err)
		return //todo, better handling
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return // todo, better handling
	}
	var info TokenResp
	err = json.Unmarshal(b, &info)
	if err != nil {
		log.Println(err)
		log.Println(string(b))
		return // todo
	}
	b2, err := ioutil.ReadFile("oauth_config.json")
	if err != nil {
		log.Println(err)
		return
	}
	var conf JSONConfig
	err = json.Unmarshal(b2, &conf)
	if err != nil {
		log.Println(err)
		return
	}
	if info.Audience != conf.Web.ClientID {
		log.Println("client id mismatch")
		return
	}
	key := RandStringRunes(keyLength)
	SetAuth(key, info.Email, time.Now().Add(24*time.Hour))
	err = CreateUser(a.db, info.Name, info.Email)
	log.Println("called createUser")
	if err != nil {
		log.Println("error in createUser ", err) // TODO better err handling
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte(key))
}

func (a app) apiUser(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}
	user, err := GetUser(a.db, email)
	if err != nil {
		handleErr(w, r, err, "unable to get user's info", http.StatusBadRequest)
		return
	}
	m := map[string]interface{}{
		"user": user,
	}
	err = json.NewEncoder(w).Encode(m)
	if err != nil {
		handleErr(w, r, err, "unable to encode response", http.StatusInternalServerError)
		return
	}
}

func (a app) apiUserTeam(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		teams, err := GetUsersTeams(a.db, email)
		if err != nil {
			handleErr(w, r, err, "unable to get user's teams", http.StatusInternalServerError)
			return
		}
		m := map[string]interface{}{
			"teams": teams,
		}
		err = json.NewEncoder(w).Encode(m)
		if err != nil {
			handleErr(w, r, err, "unable to encode response", http.StatusInternalServerError)
			return
		}
		return
	}

	var payload struct {
		Team string `json:"team"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, fmt.Sprintf(`unable to marshal body. Should be {"team":"team_name"}. Got %s`, string(b)), http.StatusBadRequest)
		return
	}
	if payload.Team == "" {
		handleErr(w, r, nil, "team name cannot be empty", http.StatusBadRequest)
		return
	}
	if r.Method == "POST" {
		err = AssignTeamToUser(a.db, email, payload.Team)
		if err != nil {
			handleErr(w, r, err, "unable to assign team to user", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	} else if r.Method == "DELETE" {
		err = RemoveTeamFromUser(a.db, email, payload.Team)
		if err != nil {
			handleErr(w, r, err, "unable to remove team from user", http.StatusInternalServerError)
			return
		}
		return
	} else {
		handleErr(w, r, nil, "unexpected method: "+r.Method, http.StatusBadRequest)
		return
	}
}

func (a app) apiUserGoal(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	var payload struct {
		Goal string `json:"goal"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, `unable to marshal body. Should be {"goal":"description"}`, http.StatusBadRequest)
		return
	}
	if payload.Goal == "" {
		handleErr(w, r, nil, "goal cannot be empty", http.StatusBadRequest)
		return
	}

	err = AssignGoalToUser(a.db, email, payload.Goal)
	if err != nil {
		handleErr(w, r, err, "unable to assign goal to user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (a app) apiUserReviewees(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	var payload struct {
		Cycle string `json:"cycle"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, `unable to marshal body. Should be {"cycle":"cycle name"}`, http.StatusBadRequest)
		return
	}
	if payload.Cycle == "" {
		handleErr(w, r, nil, "cycle cannot be empty", http.StatusBadRequest)
		return
	}

	var data struct {
		Reviewees []UserInfoLite `json:"reviewees"`
	}

	data.Reviewees, err = GetReviewees(a.db, email, payload.Cycle)
	if err != nil {
		handleErr(w, r, err, "unable to get reviewees", http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		handleErr(w, r, err, "unable to encode response", http.StatusInternalServerError)
		return
	}
}

func (a app) apiUserReviews(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		var data struct {
			Reviews []Review `json:"reviews"`
		}
		var err error
		data.Reviews, err = GetUserReviews(a.db, email)
		if err != nil {
			handleErr(w, r, err, "unable to get reviews", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			handleErr(w, r, err, "unable to encode response", http.StatusInternalServerError)
			return
		}
		return
	} else if r.Method == "POST" {
		var payload struct {
			RevieweeEmail string   `json:"reviewee_email"`
			Strengths     []string `json:"strengths"`
			Opportunities []string `json:"growth_opportunities"`
			Cycle         string   `json:"cycle"`
		}
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(b, &payload)
		if err != nil {
			handleErr(w, r, err, `unable to marshal body. Should be {"goal":"description"}`, http.StatusBadRequest)
			return
		}
		if payload.RevieweeEmail == "" || len(payload.Strengths) == 0 || len(payload.Opportunities) == 0 || payload.Cycle == "" {
			handleErr(w, r, nil, "reviewee_email, strengths, growth_opportunies, and/or cycle cannot be empty", http.StatusBadRequest)
			return
		}

		err = AddUserReview(a.db, payload.RevieweeEmail, payload.Strengths, payload.Opportunities, payload.Cycle)
		if err != nil {
			handleErr(w, r, err, "unable to add review", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	} else {
		handleErr(w, r, nil, "unexpected method "+r.Method, http.StatusBadRequest)
		return
	}
}

func (a app) apiUserReviewer(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	var payload struct {
		UserEmail string `json:"user_email"`
		Cycle     string `json:"cycle"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, `unable to marshal body. Should be {"user_email":"email", "cycle":"cycle name"}`, http.StatusBadRequest)
		return
	}
	if payload.UserEmail == "" {
		handleErr(w, r, nil, "user_email cannot be empty", http.StatusBadRequest)
		return
	}

	if payload.Cycle == "" {
		handleErr(w, r, nil, "cycle cannot be empty", http.StatusBadRequest)
		return
	}

	if r.Method != "POST" {
		handleErr(w, r, nil, "unexpected method "+r.Method, http.StatusBadRequest)
		return
	}

	err = SetUserReviewer(a.db, email, payload.UserEmail, payload.Cycle)
	if err != nil {
		handleErr(w, r, err, "unable to set reviewer", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (a app) apiAdminCycles(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var data struct {
			Cycles []Cycle `json:"cycles"`
		}
		var err error
		data.Cycles, err = GetCycles(a.db)
		if err != nil {
			handleErr(w, r, err, "unable to get cycles", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			// note, this will give an error of status code already written.
			// TODO: have a new handler that does nto set status code?
			handleErr(w, r, err, "unable to marshal payload", http.StatusInternalServerError)
			return
		}
		return
	}

	var payload struct {
		Cycle  string `json:"cycle"`
		IsOpen bool   `json:"is_open"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, `unable to marshal body. Should be {"cycle":"cycle name", "is_open":bool} (note, is_open is for PUT calls only)`, http.StatusBadRequest)
		return
	}
	if payload.Cycle == "" {
		handleErr(w, r, nil, "cycle cannot be empty", http.StatusBadRequest)
		return
	}

	if r.Method == "POST" {
		err = AddCycle(a.db, payload.Cycle)
		if err != nil {
			handleErr(w, r, err, "unable to add cycle", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	} else if r.Method == "PUT" {
		err = UpdateCycle(a.db, payload.Cycle, payload.IsOpen)
		if err != nil {
			handleErr(w, r, err, "unable to update cycle", http.StatusInternalServerError)
			return
		}
		return
	} else if r.Method == "DELETE" {
		err = DeleteCycle(a.db, payload.Cycle)
		if err != nil {
			handleErr(w, r, err, "unable to delete cycle", http.StatusInternalServerError)
			return
		}
		return
	} else {
		handleErr(w, r, nil, "unexpected method "+r.Method, http.StatusBadRequest)
		return
	}
}

func (a app) apiAdminTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var data struct {
			Teams []string `json:"teams"`
		}
		var err error
		data.Teams, err = GetTeams(a.db)
		if err != nil {
			handleErr(w, r, err, "unable to get teams", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			// note, this will give an error of status code already written.
			// TODO: have a new handler that does nto set status code?
			handleErr(w, r, err, "unable to marshal payload", http.StatusInternalServerError)
			return
		}
		return
	}

	var payload struct {
		Team string `json:"team"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, `unable to marshal body. Should be {"team":"team name"}`, http.StatusBadRequest)
		return
	}
	if payload.Team == "" {
		handleErr(w, r, nil, "team cannot be empty", http.StatusBadRequest)
		return
	}

	if r.Method == "POST" {
		err = AddTeam(a.db, payload.Team)
		if err != nil {
			handleErr(w, r, err, "unable to add team", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	} else if r.Method == "DELETE" {
		err = DeleteTeam(a.db, payload.Team)
		if err != nil {
			handleErr(w, r, err, "unable to delete team", http.StatusInternalServerError)
			return
		}
		return
	} else {
		handleErr(w, r, nil, "unexpected method "+r.Method, http.StatusBadRequest)
		return
	}
}

func handleErr(w http.ResponseWriter, r *http.Request, err error, msg string, code int) {
	// TODO get access to app's logger to get req id, path, etc for free
	_, fn, line, _ := runtime.Caller(1)
	msg = fmt.Sprintf("[ %s:%d ] %s", fn, line, msg)
	if err == nil {
		log.Printf("error: %s", msg)
	} else {
		log.Printf("error: %s - %v", msg, err)
	}
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func AuthMW(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var isAuthorized bool
		var email string
		var ok bool
		var authVal string

		cookie, err := r.Cookie("auth")
		if err == nil {
			authVal = cookie.Value
		}

		if authVal == "" {
			// check for API key
			authVal = r.Header.Get(xSessionHeader)
		}

		if email, ok = IsValidAuth(authVal); ok {
			isAuthorized = true
		}

		if !isAuthorized {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxEmail, email)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// JSONConfig is the format of the json file located
// at https://console.developers.google.com/apis/credentials
type JSONConfig struct {
	Web struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"web"`
}

// TokenResp models the server side response when validating the Google Sign In Token
type TokenResp struct {
	Audience string `json:"aud"`
	Expires  string `json:"exp"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	PicURL   string `json:"picture"`
}
