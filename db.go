package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
)

// initDB creates the db if needed
func initDB(dbfile string) error {
	if _, err := os.Stat(dbfile); os.IsNotExist(err) {
		log.Printf("creating %s", dbfile)
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

// createUser idempotently creates a user. If the user already exists, nothing happens.
func createUser(db *sql.DB, name, email string) (err error) {
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
	create table users (id integer not null primary key, name text, email text, goals text);
	create table teams (id integer not null primary key, name text);
	create table user_teams (
		id integer not null primary key,
		user_id integer, team_id integer,
		FOREIGN KEY(user_id) REFERENCES users(id),
		FOREIGN KEY(team_id) REFERENCES teams(id)
	);
	create table review_cycles (id integer not null primary key, name text, is_open boolean);
	create table reviews (
		id integer not null primary key,
		recipient_id integer,
		review_cycle_id integer,
		feedback text,
		is_strength boolean,
		is_growth_opportunity boolean,
		FOREIGN KEY(recipient_id) REFERENCES users(id),
		FOREIGN KEY(review_cycle_id) REFERENCES review_cycles(id)
	);
	create table review_requests (
		id integer not null primary key,
		recipient_id integer,
		reviewer_id integer,
		cycle_id integer,
		FOREIGN KEY (recipient_id) REFERENCES user(id),
		FOREIGN KEY (reviewer_id) REFERENCES user(id),
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
