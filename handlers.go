package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/aymerick/raymond"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
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

func dashHandler(w http.ResponseWriter, r *http.Request) {
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

func tokenHandler(w http.ResponseWriter, r *http.Request) {
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
	err = createUser(DB, info.Name, info.Email)
	log.Println("called createUser")
	if err != nil {
		log.Println("error in createUser ", err) // TODO better err handling
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte(key))
}

func apiUser(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}
	user, err := GetUser(DB, email)
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

func apiUserTeam(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		teams, err := GetUsersTeams(DB, email)
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
		handleErr(w, r, err, `unable to marshal body. Should be {"team":"team_name"}`, http.StatusBadRequest)
		return
	}
	if payload.Team == "" {
		handleErr(w, r, nil, "team name cannot be empty", http.StatusBadRequest)
		return
	}
	if r.Method == "POST" {
		err = AssignTeamToUser(DB, email, payload.Team)
		if err != nil {
			handleErr(w, r, err, "unable to assign team to user", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	} else if r.Method == "DELETE" {
		err = RemoveTeamFromUser(DB, email, payload.Team)
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

func apiUserGoal(w http.ResponseWriter, r *http.Request) {
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

	err = AssignGoalToUser(DB, email, payload.Goal)
	if err != nil {
		handleErr(w, r, err, "unable to assign goal to user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func apiUserReviewees(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	var data struct {
		Reviewees []userInfoLite `json:"reviewees"`
	}
	var err error
	data.Reviewees, err = GetReviewees(DB, email)
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

func apiUserReviews(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		var data struct {
			Reviews []review `json:"reviews"`
		}
		var err error
		data.Reviews, err = GetUserReviews(DB, email)
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
		if payload.RevieweeEmail == "" || len(payload.Strengths) == 0 || len(payload.Opportunities) == 0 {
			handleErr(w, r, nil, "reviewee_email, strengths, or growth_opportunies cannot be empty", http.StatusBadRequest)
			return
		}

		err = AddUserReview(DB, payload.RevieweeEmail, payload.Strengths, payload.Opportunities)
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

func apiUserReviewer(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(ctxEmail).(string)
	if email == "" {
		handleErr(w, r, nil, "missing email context", http.StatusInternalServerError)
		return
	}

	var payload struct {
		UserEmail string `json:"user_email"`
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleErr(w, r, err, "unable to read request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		handleErr(w, r, err, `unable to marshal body. Should be {"user_email":"email"}`, http.StatusBadRequest)
		return
	}
	if payload.UserEmail == "" {
		handleErr(w, r, nil, "user_email cannot be empty", http.StatusBadRequest)
		return
	}

	if r.Method != "POST" {
		handleErr(w, r, nil, "unexpected method "+r.Method, http.StatusBadRequest)
		return
	}

	err = SetUserReviewer(DB, email, payload.UserEmail)
	if err != nil {
		handleErr(w, r, err, "unable to set reviewer", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func apiAdminTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var data struct {
			Cycles []cycle `json:"cycles"`
		}
		var err error
		data.Cycles, err = GetCycles(DB)
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
		err = AddCycle(DB, payload.Cycle)
		if err != nil {
			handleErr(w, r, err, "unable to add cycle", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	} else if r.Method == "PUT" {
		err = UpdateCycle(DB, payload.Cycle, payload.IsOpen)
		if err != nil {
			handleErr(w, r, err, "unable to update cycle", http.StatusInternalServerError)
			return
		}
		return
	} else if r.Method == "DELETE" {
		err = DeleteCycle(DB, payload.Cycle)
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

func apiAdminCycles(w http.ResponseWriter, r *http.Request) {

}

func handleErr(w http.ResponseWriter, r *http.Request, err error, msg string, code int) {
	// TODO get access to app's logger to get req id, path, etc for free
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
