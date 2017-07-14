package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
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

type app struct {
	db *sql.DB
}

func main() {
	var dbfile string
	var port int
	flag.StringVar(&dbfile, "sqlite-path", "peerreview.db", "set the path to the sqlite3 db file")
	// TODO: consider dynamic rewriting of html/js depending on port used
	flag.IntVar(&port, "port", 3333, "set the port the server runs on. Note: the html/js needs to point to this same address. Best to leave it default.")
	flagenv.Parse()
	flag.Parse()

	a := app{}
	var err error

	err = InitDB(dbfile)
	if err != nil {
		log.Fatal(err)
	}

	a.db, err = sql.Open("sqlite3", dbfile)
	if err != nil {
		log.Fatalf("unable to open %s - %v", dbfile, err)
	}
	defer a.db.Close()

	if err := a.db.Ping(); err != nil {
		log.Fatal("unable to ping the db", err)
	}

	err = verifyDB(a.db)
	if err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("unable to create listener - %v", err)
	}
	if err := Serve(a, l, true); err != nil {
		log.Fatal(err)
	}
}

// Serve ...
func Serve(a app, l net.Listener, showLogs bool) error {
	logger := logrus.New()
	logger.Formatter = &logrus.TextFormatter{
	// disable, as we set our own
	// DisableTimestamp: true,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	if showLogs {
		r.Use(NewStructuredLogger(logger))
	}
	r.Use(middleware.Recoverer)

	// separate route created for this, intended to prevent logging of its request
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		// TOOD, serve favicon
	})

	r.Get("/", a.rootHandler)
	r.With(AuthMW).Get("/dash", a.dashHandler)
	r.Post("/tokensignin", a.tokenHandler)

	r.Mount("/api", apiRouter(a))

	// if you update the port, you have to update the Google Sign In Client
	// at https://console.developers.google.com/apis/credentials
	//log.Printf("listening on :%d", l.Addr().(*net.TCPAddr).Port)

	if err := http.Serve(l, r); err != nil {
		return nil
	}
	return nil
}

func apiRouter(a app) http.Handler {
	r := chi.NewRouter()
	r.Use(AuthMW)
	r.Get("/user/team", a.apiUserTeam)
	r.Post("/user/team", a.apiUserTeam)
	r.Delete("/user/team", a.apiUserTeam)

	r.Post("/user/goal", a.apiUserGoal)

	r.Get("/user/reviewees/{cycleName}", a.apiUserReviewees)

	r.Get("/user/reviews", a.apiUserReviews)
	r.Post("/user/reviews", a.apiUserReviews)

	r.Post("/user/reviewer", a.apiUserReviewer)

	r.Get("/user", a.apiUser)

	r.Get("/admin/cycles", a.apiAdminCycles)
	r.Post("/admin/cycles", a.apiAdminCycles)
	r.Put("/admin/cycles", a.apiAdminCycles)
	r.Delete("/admin/cycles", a.apiAdminCycles)

	r.Get("/admin/teams", a.apiAdminTeams)
	r.Post("/admin/teams", a.apiAdminTeams)
	r.Delete("/admin/teams", a.apiAdminTeams)

	return r
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
GET     /api/user                           {"user":{"name": $name, "email", $email, "goal":$goal, "teams":[$team_a]}}
GET     /api/user/team                      {"teams":[$team_a, $team_b, $team_c]}
POST    /api/user/team   {"team": $team}    201
DELETE  /api/user/team   {"team": $team}    200
POST    /api/user/goal   {"goal": $goal}    201

submit review page
user can see other team members (name). When they click on a team member, they can enter multiple feedbacks under strength or growth is_growth_opportunity
they user is told that the feedback is anonymous and after they submit, the cannot edit their feedback, but they can provide additional feedback if they wish. They can choose to sign their name.

Resource                     Payload                                                                                                        Response
GET     /api/user/reviewees/:$cycle_name                                                                                                    {"reviewees": [{"name": $name, "email": $email}]} # this will populate with anyone on the same team and anyone who has requested a review from this user during this cycle
POST    /api/user/reviews    {"reviewee_email":$email, "strengths":[$strength], "growth_opportunities":[$opportunity], "cycle": $cycle_name}  201

they can also view users who have requested that the signed in user review them (good for cross team review)

Request Review

Page will have autocomplete of folks who have signed up. These requests are for those outside your team to give them visability to review you. Pending: notification of review request.

Resource                 Payload                                 Response
POST /api/user/reviewer  {"user_email": $user, "cycle": $cycle}  201

view reviews page
sorted by review cycle, the shows the reviews by strength or growth opportunity

Resource Payload Response
GET /api/user/reviews   {"reviews":[{"cycle":$cycle, "strengths":[$strength], "growth_opportunities":[$opportunity]}}

Admin stuffs
GET    /api/admin/cycles                                  {"cycles":[{"name":$cycle_name, "is_open":bool}]}
POST   /api/admin/cycles {"cycle":$name}                  201
PUT    /api/admin/cycles {"cycle":$name, "is_open":bool}  200
DELETE /api/admin/cycles {"cycle":$name}                  200

GET    /api/admin/teams                                   {"teams":[$team_name]}
POST   /api/admin/teams  {"team":$team_name}              201
DELETE /api/admin/teams  {"team":$team_name}              200

for adding teams... show a list of teams. Have link, team not listed? add it! with form.

Operability: set up db back ups. Capture error logs. v2: email error reports?

whitelabel domains? domains -> teams?
*/
