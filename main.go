package main

import (
	"database/sql"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/facebookgo/flagenv"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

const schemaVersion = "2017-07-03-07:22"
const keyLength = 36
const xSessionHeader = "x-session-token"

type ctxType string

var ctxEmail ctxType = "email"

// DB is globally available
var DB *sql.DB

func init() {
	if _, err := os.Stat("oauth_config.json"); err != nil {
		log.Fatal("oauth_config.json not found. Download the file contents from https://console.developers.google.com/apis/credentials. See README.md for more details.")
	}
	InitAuth()
	go func() {
		for _ = range time.Tick(5 * time.Minute) {
			PruneAuth()
		}
	}()
}

func main() {
	var dbfile string
	flag.StringVar(&dbfile, "sqlite-path", "peerreview.db", "set the path to the sqlite3 db file")
	flagenv.Parse()
	flag.Parse()

	var err error

	err = initDB(dbfile)
	if err != nil {
		log.Fatal(err)
	}

	DB, err = sql.Open("sqlite3", dbfile)
	if err != nil {
		log.Fatalf("unable to open %s - %v", dbfile, err)
	}
	defer DB.Close()

	if err := DB.Ping(); err != nil {
		log.Fatal("unable to ping the db", err)
	}

	err = verifyDB(DB)
	if err != nil {
		log.Fatal(err)
	}

	logger := logrus.New()
	logger.Formatter = &logrus.TextFormatter{
	// disable, as we set our own
	// DisableTimestamp: true,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(NewStructuredLogger(logger))
	r.Use(middleware.Recoverer)

	// separate route created for this, intended to prevent logging of its request
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		// TOOD, serve favicon
	})

	r.Get("/", rootHandler)
	r.With(AuthMW).Get("/dash", dashHandler)
	r.Post("/tokensignin", tokenHandler)

	// if you update the port, you have to update the Google Sign In Client
	// at https://console.developers.google.com/apis/credentials
	log.Println("listening on :3333")
	if err := http.ListenAndServe(":3333", r); err != nil {
		log.Fatal(err)
	}
}

func RandStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.=-_")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

/*
Planning:
Schemas:

users
id name email goals

teams
id name

user_teams
id user_id team_id

reviews
id recipient_id review_cycle_id feedback is_strength is_growth_opportunity

review_cycles
id name is_open

review_requests
id recipient_id reviewer_id cycle_id

Workflow:
user signs in with google.

Pages and API:
settings page
they can adjust what team(s) they are on. Some managers have multiple teams, qa can have multiple teams. Many folks have one team.
can set a goal description

Resource                 Payload            Response
GET     /api/teams                          {"teams":[$team_a, $team_b, $team_c]}
GET     /api/user                           {"user":{"name": $name, "email", $email, "goal":$goal, "teams":[$team_a]}}
POST    /api/user/team   {"team": $team}    201
DELETE  /api/user/team   {"team": $team}    200
POST    /api/user/goal   {"goal": $goal}    201

submit review page
user can see other team members (name). When they click on a team member, they can enter multiple feedbacks under strength or growth is_growth_opportunity
they user is told that the feedback is anonymous and after they submit, the cannot edit their feedback, but they can provide additional feedback if they wish. They can choose to sign their name.

Resource                     Payload                                                                           Response
GET     /api/user/reviewees                                                                                    {"names": [$name_1, $name_2]} # this will populate with anyone on the same team and anyone who has requested a review from this user
POST    /api/reviews         {"reviewee":$user, "strengths":[$strength], "growth_opportunity":[$opportunity]}  201

they can also view users who have requested that the signed in user review them (good for cross team review)

Request Review

Page will have autocomplete of folks who have signed up. These requests are for those outside your team to give them visability to review you. Pending: notification of review request.

Resource                 Payload          Response
POST /api/user/reviewer  {"user": $user}  201

view reviews page
sorted by review cycle, the shows the reviews by strength or growth opportunity

Resource Payload Response
GET /api/reviews   {"reviews":[{"name":$cycle, "strengths":[$strength], "growth_opportunity":[$opportunity]}}

for adding teams... show a list of teams. Have link, team not listed? add it! with form.

Operability: set up db back ups. Capture error logs. v2: email error reports?

whitelabel domains? domains -> teams?
*/
