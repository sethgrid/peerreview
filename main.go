package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/facebookgo/flagenv"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

const schemaVersion = "2017-07-03-07:22"

func createDB(path string) error {
	var db *sql.DB
	var err error

	db, err = sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalf("unable to open %s - %v", path, err)
	}
	defer db.Close()
	defer func() {
		if err != nil {
			err = os.Remove(path)
			if err != nil {
				log.Println("unable to remove database file ", err)
			}
		}
	}()

	q := `
	create table schema_version (version text not null primary key);
	create table users (id integer not null primary key, name text, email text, goals text);
	create table teams (id integer not null primary key, name text);
	create table user_teams (id integer not null primary key, user_id integer, team_id integer, FOREIGN KEY(user_id) REFERENCES users(id), FOREIGN KEY(team_id) REFERENCES teams(id));
	create table review_cycles (id integer not null primary key, name text, is_open boolean);
	create table reviews (id integer not null primary key, recipient_id integer, review_cycle_id integer, feedback text, is_strength boolean, is_growth_opportunity boolean, FOREIGN KEY(recipient_id) REFERENCES users(id), FOREIGN KEY(review_cycle_id) REFERENCES review_cycles(id));
	create table review_requests (id integer not null primary key, recipient_id integer, reviewer_id integer, cycle_id integer, FOREIGN KEY (recipient_id) REFERENCES user(id), FOREIGN KEY (reviewer_id) REFERENCES user(id), FOREIGN KEY (cycle_id) REFERENCES review_cycles(id));
	`
	_, err = db.Exec(q, schemaVersion)
	if err != nil {
		return errors.Wrapf(err, "query: %q", q)
	}

	q = "insert into schema_version (version) values (?)"
	_, err = db.Exec(q, schemaVersion)
	if err != nil {
		return errors.Wrapf(err, "query: %q", q)
	}
	return nil
}

func main() {
	var dbfile string
	flag.StringVar(&dbfile, "sqlite-path", "peerreview.db", "set the path to the sqlite3 db file")
	flagenv.Parse()
	flag.Parse()

	if _, err := os.Stat(dbfile); os.IsNotExist(err) {
		log.Printf("creating %s", dbfile)
		err := createDB(dbfile)
		if err != nil {
			log.Fatal("unable to create db", err)
		}
	} else if err != nil {
		log.Fatal("unexpected error looking for sqlite3 db file", err)
	} else {
		log.Printf("%s found", dbfile)
	}

	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		log.Fatalf("unable to open %s - %v", dbfile, err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("unable to ping the db", err)
	}

	row := db.QueryRow("select version from schema_version")
	var detectedSchemaVersion string
	err = row.Scan(&detectedSchemaVersion)
	if err != nil {
		log.Fatalf("unable to determine schema version - %v", err)
	}
	if detectedSchemaVersion != schemaVersion {
		log.Fatalf("schema version has changed. Detected schema version: %q. App's schema version: %q. Remove or migrate the database.", detectedSchemaVersion, schemaVersion)
	}

	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	log.Println("listening on :3333")
	http.ListenAndServe(":3333", r)

	log.Println("completed!")
}

/*
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

settings page
they can adjust what team(s) they are on. Some managers have multiple teams, qa can have multiple teams. Many folks have one team.
can set a goal description

submit review page
user can see other team members (gravatar + name). When they click on a team member, they can enter multiple feedbacks under strength or growth is_growth_opportunity
they user is told that the feedback is anonymous and after they submit, the cannot edit their feedback, but they can provide additional feedback if they wish. They can choose to sign their name.

they can also view users who have requested that the signed in user review them (good for cross team review)

view reviews page
sorted by review cycle, the shows the reviews by strength or growth opportunity

for adding teams... show a list of teams. Have link, team not listed? add it! with form.

Operability: set up db back ups. Capture error logs. v2: email error reports?

whitelabel domains? domains -> teams?

TODO: set up a schema version for compatibility checks when things get upgraded
*/
