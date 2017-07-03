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
