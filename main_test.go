package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

var preserveTestDB bool
var showLogs bool
var randseed int64

func init() {
	flag.BoolVar(&preserveTestDB, "save-db", false, "set to save the test database for debugging")
	flag.BoolVar(&showLogs, "show-logs", false, "set to show logs")
	flag.Int64Var(&randseed, "seed", time.Now().Unix(), "set seed to ensure given random values")
	flag.Parse()
	fmt.Printf("Using seed %d\n", randseed)
	fmt.Println("Optional test flags: -randseed :int -save-db :bool -show-logs :bool")
}

func TestAPITeam(t *testing.T) {
	/*
		Verify no teams are assigned by default
		Verify we can insert teams
		Verify we can assign teams to a user
	*/
	cli, teardown := setupInstance()
	defer teardown()

	teams, err := cli.GetUsersTeams()
	NoErr(t, err, "Get user teams")

	if got, want := len(teams), 0; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}

	NoErr(t, cli.InsertTeam("team_a"), "Insert team a")
	NoErr(t, cli.InsertTeam("team_b"), "Insert team b")

	NoErr(t, cli.AssignTeamToUser("team_a"), "Assign team a")
	NoErr(t, cli.AssignTeamToUser("team_b"), "Assign team b")

	teams, err = cli.GetUsersTeams()
	NoErr(t, err, "Get users after insert")

	if got, want := len(teams), 2; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}
}

func NoErr(t *testing.T, err error, msg ...string) {
	if err != nil && msg != nil {
		message := strings.Join(msg, " ")
		t.Errorf("%v - %s", err, message)
	} else if err != nil {
		t.Error(err)
	}
}

func setupInstance() (*Client, func() error) {
	r := rand.New(rand.NewSource(randseed))
	testDB := fmt.Sprintf(".test_db_%d_%d", time.Now().Unix(), r.Intn(100))
	err := InitDB(testDB)
	if err != nil {
		log.Fatalf("unable to create test db - %v", err)
	}

	a := app{}
	a.db, err = sql.Open("sqlite3", testDB)
	if err != nil {
		log.Fatalf("unable to open %s - %v", testDB, err)
	}

	err = verifyDB(a.db)
	if err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("unable to create listener - %v", err)
	}

	go Serve(a, l, showLogs)
	port := l.Addr().(*net.TCPAddr).Port

	key := testDB
	email := testDB + "@example.com"
	SetAuth(key, email, time.Now().Add(24*time.Hour))
	err = CreateUser(a.db, "Test User", email)
	if err != nil {
		log.Fatalf("unable to create test user - %v", err)
	}

	cli := NewClient(fmt.Sprintf("http://localhost:%d", port), key)

	return cli, func() error {
		if preserveTestDB {
			log.Println("keeping db " + testDB)
		} else {
			if err := os.Remove(testDB); err != nil {
				log.Printf("unable to remove test db: %s - %v", testDB, err)
				return err
			}
		}
		err = a.db.Close()
		if err != nil {
			log.Printf("unable to close db - %v", err)
			return err
		}
		return nil
	}
}
