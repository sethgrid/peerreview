// main - but maybe soon to be an internal db package
/*
 Some of the methods in this package would benefit from a shared transaction or stronger (multiple) indexes on the schema,
 such as looking up the list of a user's teams, comparing, and then optionally assigning the new team.
 It is considered a known edge case that a user could concurrently try to add the same team twice
 causing a duplicate entry in the user_teams db.

 sql formatted with http://www.dpriver.com/pp/sqlformat.htm
*/
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// InitDB creates the db if needed
func InitDB(dbfile string) error {
	if _, err := os.Stat(dbfile); os.IsNotExist(err) {
		err := createDB(dbfile)
		if err != nil {
			return errors.Wrap(err, "unable to create db")
		}
	} else if err != nil {
		return errors.Wrap(err, "unexpected error looking for sqlite3 db file")
	} else {
		log.Printf("%s found", dbfile)
	}

	return nil
}

// CreateUser idempotently creates a user. If the user already exists, nothing happens.
func CreateUser(db *sql.DB, name, email string) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "unable to begin tx for createUser")
	}

	defer func() {
		if err != nil {
			// attempt a rollback and return the original error
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			err = errors.Wrap(err, "error committing tx on createUser")
		}
	}()

	row := tx.QueryRow("select id from users where email=? limit 1", email)
	var id int
	err = row.Scan(&id)
	if err == sql.ErrNoRows || id == 0 {
		_, err := tx.Exec("insert into users (name, email) values (?,?)", name, email)
		if err != nil {
			return errors.Wrap(err, "unable to create user")
		}
	} else if err != nil {
		return errors.Wrap(err, "unexpected error in createUser")
	}
	return nil
}

// GetUsersTeams gets the teams that a user is on. Usually this will be one team, but some people have multiple teams.
func GetUsersTeams(db *sql.DB, email string) ([]string, error) {
	q := `
        SELECT t.NAME
        FROM   teams t
        JOIN   user_teams ut
        ON     ut.team_id=t.id
        JOIN   users u
        ON     ut.user_id=u.id
        WHERE  u.email=?
    `
	rows, err := db.Query(q, email, email)
	if err != nil {
		return nil, errors.Wrap(err, "unable to query GetUsersTeams")
	}
	var teams []string
	for rows.Next() {
		var team string
		if err = rows.Scan(&team); err != nil {
			return nil, errors.Wrap(err, "unable to scan GetUsersTeams")
		}
		teams = append(teams, team)
	}
	if rows.Err() != nil {
		return teams, errors.Wrap(err, "error post scanning in GetUsersTeams")
	}
	return teams, nil
}

// UserInfo contains basic user information
type UserInfo struct {
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Goals string   `json:"goal"`
	Teams []string `json:"teams"`
}

// UserInfoLite is a subset of UserInfo
type UserInfoLite struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// queryPP is a query (pretty) printer, helpful for logging/deubbing.
// it takes a parameterized query and outputs a query that is pastable into the db console.
func queryPP(q string, args ...string) string {
	var i int
	for strings.IndexAny(q, "?") != -1 {
		q = strings.Replace(q, "?", fmt.Sprintf(`"%s"`, args[i]), 1)
	}
	return q
}

// GetUser returns basic user information given a user's email
func GetUser(db *sql.DB, email string) (UserInfo, error) {
	// could re-use GetUsersTeams below. Good for code re-use, but I wanted to play with NextResultSet().
	// In this application, saving a new query to the db wont mean much, so, "meh."
	info := UserInfo{}
	q := `
        SELECT name,
               goals
        FROM   users
        WHERE  email=?;
	`

	rows, err := db.Query(q, email, email)
	if err != nil {
		return info, errors.Wrap(err, "unable to query GetUser")
	}
	for rows.Next() {
		var name, goals string
		if err = rows.Scan(&name, &goals); err != nil {
			return info, errors.Wrap(err, "unable to scan GetUser first result set")
		}
		info.Name = name
		info.Email = email
		info.Goals = goals
	}
	if rows.Err() != nil {
		return info, errors.Wrap(err, "error post scan in GetUser")
	}

	info.Teams, err = GetUsersTeams(db, email)
	if err != nil {
		return info, err
	}

	return info, nil
}

// AssignTeamToUser links a user to a given team
func AssignTeamToUser(db *sql.DB, email string, team string) error {
	teams, err := GetUsersTeams(db, email)
	if err != nil {
		return errors.Wrap(err, "unable to fetch teams for comparison in AssignTeamToUser")
	}
	if inList(team, teams) {
		// user already associated with this team
		return nil
	}

	q := `
        INSERT INTO user_teams
                    (user_id,
                    team_id)
        VALUES      ((SELECT id
                    FROM     users
                    WHERE    email =?
                    LIMIT  1),
                    (SELECT id
                    FROM     teams
                    WHERE    name =?
                    LIMIT  1))
    `
	if _, err := db.Exec(q, email, team); err != nil {
		return errors.Wrap(err, "unable to assign team in AssignTeamToUser")
	}
	return nil
}

// RemoveTeamFromUser unlinks a user from a given team
func RemoveTeamFromUser(db *sql.DB, email string, team string) error {
	teams, err := GetUsersTeams(db, email)
	if err != nil {
		return errors.Wrap(err, "unable to fetch teams for comparison in RemoveTeamFromUser")
	}
	if !inList(team, teams) {
		// if the user does not have that team already that is fine, return nil
		return nil
	}

	// TODO - look into recompiling sqlite3
	q := `
    DELETE FROM user_teams
    WHERE  user_id = (SELECT id
                    FROM   users
                    WHERE  email =?
                    LIMIT  1)
        AND team_id = (SELECT id
                        FROM   teams
                        WHERE  name =?
                        LIMIT  1)
    -- LIMIT 1 requires sqlite to be compiled with #define SQLITE_ENABLE_UPDATE_DELETE_LIMIT
    `
	if _, err := db.Exec(q, email, team); err != nil {
		return errors.Wrap(err, "unable to delete user-team link in RemoveTeamFromUser")
	}
	return nil
}

// AssignGoalToUser sets the goal that the user wishes other reviewers to know about themselves
func AssignGoalToUser(db *sql.DB, email string, goal string) error {
	q := "update users set goals=? where email=?"
	if _, err := db.Exec(q, goal, email); err != nil {
		return errors.Wrap(err, "unable to set user goal in AssignGoalToUser")
	}
	return nil
}

// SetUserReviewer allows a user to be reviewed by a given reviewer during a given cycle
// This link will allow a reviewer to see other potential reviewees than just team members.
// This allows for cross team reviews.
func SetUserReviewer(db *sql.DB, userEmail string, eligibleReviewer string, cycle string) error {
	q := `
    INSERT INTO review_requests
                (recipient_id,
                reviewer_id,
                cycle_id)
    VALUES      ((SELECT id
                FROM   users
                WHERE  email =?
                LIMIT  1),
                (SELECT id
                FROM   users
                WHERE  email =?
                LIMIT  1),
                (SELECT id
                FROM   review_cycles
                WHERE  name =?
                LIMIT  1))
    `
	if _, err := db.Exec(q, userEmail, eligibleReviewer, cycle); err != nil {
		return errors.Wrap(err, "unable to set review request in SetUserReviewer")
	}
	return nil
}

// GetReviewees returns a list of people for which a given user can enter a review.
// This is the user's team and any any person who has requested a review in the current cycle.
func GetReviewees(db *sql.DB, email string, cycle string) ([]UserInfoLite, error) {
	var uil []UserInfoLite
	q := `
        SELECT  name,
                email
        FROM    users
                JOIN user_teams
                    ON users.id = user_teams.user_id
        WHERE   team_id = (SELECT team_id
                        FROM user_teams
                        JOIN users
                          ON user_teams.user_id=users.id
                        WHERE  users.email = ?
                        )
			    AND email <> ?
	`

	rows, err := db.Query(q, email, email)
	if err != nil {
		return uil, errors.Wrap(err, "unable to query for team mates in GetReviewees")
	}
	for rows.Next() {
		var name, email string
		if err = rows.Scan(&name, &email); err != nil {
			return uil, errors.Wrap(err, "unable to scan for team mates in GetReviewees")
		}
		uil = append(uil, UserInfoLite{Name: name, Email: email})
	}
	if rows.Err() != nil {
		return uil, errors.Wrap(err, "error post scan q1 in GetReviewees")
	}

	q = `
        SELECT users.name,
               users.email
        FROM   users
               JOIN review_requests
                 ON reviewer_id = users.id
               JOIN review_cycles
                 ON review_requests.cycle_id = review_cycles.id
        WHERE  review_requests.recipient_id = (SELECT id
                                               FROM   users
                                               WHERE  email =?
                                              )
               AND review_cycles.name =?;
    `
	rows, err = db.Query(q, email, cycle)
	if err != nil {
		return uil, errors.Wrap(err, "unable to query for reviewers in GetReviewees")
	}

	for rows.Next() {
		var name, email string
		if err = rows.Scan(&name, &email); err != nil {
			return uil, errors.Wrap(err, "unable to scan for reviewers in GetReviewees")
		}
		uil = append(uil, UserInfoLite{Name: name, Email: email})
	}
	if rows.Err() != nil {
		return uil, errors.Wrap(err, "error post scan q2 in GetReviewees")
	}

	return uil, nil
}

// Review holds the information needed for displaying reviews
type Review struct {
	Cycle         string   `json:"cycle"`
	Strengths     []string `json:"strengths"`
	Opportunities []string `json:"growth_opportunities"`
}

// GetUserReviews gets all the reviews for a user
func GetUserReviews(db *sql.DB, email string) ([]Review, error) {
	q := `
    SELECT review_cycles.name,
           reviews.feedback,
           reviews.is_strength,
           reviews.is_growth_opportunity
    FROM   reviews
           JOIN users
             ON reviews.recipient_id = users.id
           JOIN review_cycles
             ON review_cycles.id = reviews.review_cycle_id
    WHERE  users.email = ?;
    `
	rows, err := db.Query(q, email)
	if err != nil {
		return nil, errors.Wrap(err, "unable to query reviews")
	}

	// reviews will be the return value.
	// todo: sorted? end result is json which is unsorted, so prolly not.
	var reviews []Review

	// m allows for easier record keeping as we scan multiple rows back
	// it will be read into the reviews slice after we've collected all feedback
	m := make(map[string]Review)

	for rows.Next() {
		var cycleName, feedback string
		var isStrength, isOpportunity bool
		// might have to read in int and treat as bool
		if err = rows.Scan(&cycleName, &feedback, &isStrength, &isOpportunity); err != nil {
			return nil, errors.Wrap(err, "unable to scan reviews")
		}
		r := m[cycleName]
		r.Cycle = cycleName
		if isStrength {
			r.Strengths = append(r.Strengths, feedback)
		}
		if isOpportunity {
			r.Opportunities = append(r.Opportunities, feedback)
		}
		m[cycleName] = r
	}
	if rows.Err() != nil {
		return nil, errors.Wrap(err, "error post scan in GetUserReviews")
	}

	for _, v := range m {
		reviews = append(reviews, v)
	}

	return reviews, nil
}

// AddUserReview inserts a new review into the system for a given cycle for the given recipient
// Note that there is no link to the reviewer. This ensures that we have anonymous feedback.
func AddUserReview(db *sql.DB, revieweeEmail string, strengths []string, opportunities []string, cycle string) error {
	q := `
    INSERT INTO reviews
            (recipient_id,
             review_cycle_id,
             feedback,
             is_strength,
             is_growth_opportunity)
    VALUES  ((SELECT id
              FROM   users
              WHERE  email =?
             LIMIT  1),
             (SELECT id
              FROM   review_cycles
              WHERE  name =? ),
             ?,
             ?,
             ?) ;
    `
	// could make some uber query, but it is just easier to iterate
	for _, strength := range strengths {
		if _, err := db.Exec(q, revieweeEmail, cycle, strength, true, false); err != nil {
			return errors.Wrap(err, "unable to insert strengths in reviews")
		}
	}
	for _, opportunity := range opportunities {
		if _, err := db.Exec(q, revieweeEmail, cycle, opportunity, false, true); err != nil {
			return errors.Wrap(err, "unable to insert opportunity in reviews")
		}
	}
	return nil
}

// Cycle holds basic info about a cycle (name / is open)
type Cycle struct {
	Name   string `json:"name"`
	IsOpen bool   `json:"is_open"`
}

// GetCycles returns all cycles
func GetCycles(db *sql.DB) ([]Cycle, error) {
	var cycles []Cycle
	q := `select name, is_open from review_cycles`
	rows, err := db.Query(q)
	if err != nil {
		return nil, errors.Wrap(err, "unable to query review cycles")
	}
	for rows.Next() {
		var name string
		var isOpen bool
		if err = rows.Scan(&name, &isOpen); err != nil {
			return nil, errors.Wrap(err, "unable to scan review cycles")
		}
		cycles = append(cycles, Cycle{Name: name, IsOpen: isOpen})
	}
	if rows.Err() != nil {
		return cycles, errors.Wrap(rows.Err(), "error post scan in GetCycles")
	}
	return cycles, nil
}

// AddCycle adds it if it does not yet exist
func AddCycle(db *sql.DB, cycleName string) error {
	cycles, err := GetCycles(db)
	if err != nil {
		return errors.Wrap(err, "unable to get cycles for comparison when adding cycles")
	}
	var found bool
	for _, cycle := range cycles {
		if cycle.Name == cycleName {
			found = true
			break
		}
	}
	if found {
		return nil
	}
	q := "insert into review_cycles (name, is_open) values (?, ?)"
	if _, err := db.Exec(q, cycleName, true); err != nil {
		return errors.Wrap(err, "unable to insert new review cycle")
	}
	return nil
}

// UpdateCycle sets if the cycle is open or not
func UpdateCycle(db *sql.DB, cycleName string, isOpen bool) error {
	q := "update review_cycles set is_open=? where name=?"
	if _, err := db.Exec(q, bool2int(isOpen), cycleName); err != nil {
		return errors.Wrap(err, "unable to update cycle")
	}
	return nil
}

// DeleteCycle removes a cycle. Due to foreign key constraints, it will fail if it is in use.
func DeleteCycle(db *sql.DB, cycleName string) error {
	cycles, err := GetCycles(db)
	if err != nil {
		return errors.Wrap(err, "unable to get cycles for comparison when deleting")
	}
	// if it is not there, don't need to delete it
	var found bool
	for _, cycle := range cycles {
		if cycle.Name == cycleName {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	q := "delete from review_cycles where name=?"
	if _, err := db.Exec(q, cycleName, true); err != nil {
		return errors.Wrap(err, "unable to delete review cycle")
	}
	return nil
}

// GetTeams returns all Teams
func GetTeams(db *sql.DB) ([]string, error) {
	var teams []string
	q := `select name from teams`
	rows, err := db.Query(q)
	if err != nil {
		return nil, errors.Wrap(err, "unable to query teams")
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errors.Wrap(err, "unable to scan teams")
		}
		teams = append(teams, name)
	}
	if rows.Err() != nil {
		return teams, errors.Wrap(rows.Err(), "error post scan in GetTeams")
	}
	return teams, nil
}

// AddTeam adds it if it does not yet exist
func AddTeam(db *sql.DB, teamName string) error {
	teams, err := GetTeams(db)
	if err != nil {
		return errors.Wrap(err, "unable to get teams for comparison when adding teams")
	}
	if inList(teamName, teams) {
		return nil
	}
	q := "insert into teams (name) values (?)"
	if _, err := db.Exec(q, teamName); err != nil {
		return errors.Wrap(err, "unable to insert new team")
	}
	return nil
}

// DeleteTeam removes a Team. Due to foreign key constraints, it will fail if it is in use.
func DeleteTeam(db *sql.DB, teamName string) error {
	teams, err := GetTeams(db)
	if err != nil {
		return errors.Wrap(err, "unable to get teams for comparison when deleting")
	}
	if !inList(teamName, teams) {
		// if it is not there, don't need to delete it
		return nil
	}
	q := "delete from teams where name=?"
	if _, err := db.Exec(q, teamName, true); err != nil {
		return errors.Wrap(err, "unable to delete review team")
	}
	return nil
}

// verifyDB makes sure that the current schema version is the same as the installed schema version
func verifyDB(db *sql.DB) error {
	row := db.QueryRow("select version from schema_version")
	var detectedSchemaVersion string
	err := row.Scan(&detectedSchemaVersion)
	if err != nil {
		return errors.Wrap(err, "unable to determine schema version")
	}
	if detectedSchemaVersion != schemaVersion {
		return fmt.Errorf("schema version has changed. Detected schema version: %q. App's schema version: %q. Remove or migrate the database", detectedSchemaVersion, schemaVersion)
	}

	return nil
}

// createDB initialized the schema. It also sets the schema version used for later validation when the service starts and the db already exists.
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
    create table users (
		id integer not null primary key,
		name text not null default "",
		email text not null,
		goals text not null default ""
	);
    create table teams (
		id integer not null primary key,
		name text not null
	);
    create table user_teams (
        id integer not null primary key,
        user_id integer not null,
		team_id integer not null,
        FOREIGN KEY(user_id) REFERENCES users(id),
        FOREIGN KEY(team_id) REFERENCES teams(id)
    );
    create table review_cycles (
		id integer not null primary key,
		name text not null,
		is_open boolean not null
	);
    create table reviews (
        id integer not null primary key,
        recipient_id integer not null,
        review_cycle_id integer not null,
        feedback text not null,
        is_strength boolean not null,
        is_growth_opportunity boolean not null,
        FOREIGN KEY(recipient_id) REFERENCES users(id),
        FOREIGN KEY(review_cycle_id) REFERENCES review_cycles(id)
    );
    create table review_requests (
        id integer not null primary key,
        recipient_id integer not null,
        reviewer_id integer not null,
        cycle_id integer not null,
        FOREIGN KEY (recipient_id) REFERENCES users(id),
        FOREIGN KEY (reviewer_id) REFERENCES users(id),
        FOREIGN KEY (cycle_id) REFERENCES review_cycles(id)
    );
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

// inList searches for a needle in a haystack
func inList(needle string, haystack []string) bool {
	for _, element := range haystack {
		if needle == element {
			return true
		}
	}
	return false
}

// bool2int converts bool to 1 or 0
func bool2int(t bool) int {
	if t {
		return 1
	}
	return 0
}

// int2bool converts any positive integer to true, and all others to false
func int2bool(i int) bool {
	if i >= 1 {
		return true
	}
	return false
}
