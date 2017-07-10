package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"runtime"
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
	fmt.Println("Optional test flags: -randseed :int -save-db :bool -show-logs :bool\n")
}

func TestAPIAdminTeams(t *testing.T) {
	/*
		Verify we can insert teams into the system
		Verify we can get teams inserted into the system
		Verify we can delete teams from the system
	*/
	cli, teardown := setupInstance()
	defer teardown()

	NoErr(t, cli.InsertTeam("team_a"), "Insert team a")
	NoErr(t, cli.InsertTeam("team_b"), "Insert team b")

	teams, err := cli.GetTeams()
	NoErr(t, err, "Get teams after insert")

	if got, want := len(teams), 2; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}

	NoErr(t, cli.DeleteTeam("team_b"), "delete team b")

	teams, err = cli.GetTeams()
	NoErr(t, err, "Get teams after insert")

	if got, want := len(teams), 1; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}
}

func TestAPIAdminCycles(t *testing.T) {
	/*
		Verify we get no cycles by default
		Verify we can add cycles
		Verify we can delete cycles
		Verify we can edit (open/close) cycles
	*/
	cli, teardown := setupInstance()
	defer teardown()

	cycles, err := cli.GetCycles()
	NoErr(t, err, "getting cycles")

	if got, want := len(cycles), 0; got != want {
		t.Errorf("got %d cycles, want %d", got, want)
	}

	NoErr(t, cli.AddCycle("cycle_1"), "adding cycle 1")
	NoErr(t, cli.AddCycle("cycle_2"), "adding cycle 2")
	NoErr(t, cli.AddCycle("cycle_3"), "adding cycle 3")

	cycles, err = cli.GetCycles()
	NoErr(t, err, "getting cycles after adding them")

	if got, want := len(cycles), 3; got != want {
		t.Errorf("got %d cycles, want %d", got, want)
	}

	NoErr(t, cli.DeleteCycle("cycle_3"), "deleting cycle")
	NoErr(t, cli.EditCycle("cycle_2", false), "edit cycle")

	cycles, err = cli.GetCycles()
	NoErr(t, err, "getting cycles after adding them")

	if got, want := len(cycles), 2; got != want {
		t.Errorf("got %d cycles, want %d", got, want)
	}

	for _, cycle := range cycles {
		if cycle.Name == "cycle_1" && cycle.IsOpen != true {
			t.Errorf("cycle 1 is closed, should be open")
		}
		if cycle.Name == "cycle_2" && cycle.IsOpen != false {
			t.Errorf("cycle 1 is open, should be closed")
		}
	}

}
func TestAPIUserTeam(t *testing.T) {
	/*
		Verify no teams are assigned by default
		Verify we can assign teams to a user
		Verify we can remove a user from a team
	*/
	cli, teardown := setupInstance()
	defer teardown()

	teams, err := cli.GetUsersTeams()
	NoErr(t, err, "Get user teams")

	if got, want := len(teams), 0; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}

	// teams must be inserted to be able to be assigned to a user
	NoErr(t, cli.InsertTeam("team_a"), "Insert team a")
	NoErr(t, cli.InsertTeam("team_b"), "Insert team b")

	NoErr(t, cli.AssignTeamToUser("team_a"), "Assign team a")
	NoErr(t, cli.AssignTeamToUser("team_b"), "Assign team b")

	teams, err = cli.GetUsersTeams()
	NoErr(t, err, "Get users after insert")

	if got, want := len(teams), 2; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}

	NoErr(t, cli.RemoveTeamFromUser("team_b"), "remove team b from user")

	teams, err = cli.GetUsersTeams()
	NoErr(t, err, "Get users after insert")

	if got, want := len(teams), 1; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}

	// verify that it works with the user endpoint too
	info, err := cli.GetUserInfo()
	NoErr(t, err, "getting user info")
	if got, want := len(info.Teams), 1; got != want {
		t.Errorf("got %d teams, want %d", got, want)
	}
}

func TestAPIUserGoal(t *testing.T) {
	cli, teardown := setupInstance()
	defer teardown()

	goal, err := cli.GetUsersGoal()
	NoErr(t, err, "get user goal")

	cli.InsertTeam("foo")
	cli.AssignTeamToUser("foo")

	if goal != "" {
		t.Errorf("got goal %q, expected no goal", goal)
	}

	expectedGoal := "I want to make awesome things"
	NoErr(t, cli.SetUserGoal(expectedGoal), "setting goal")

	goal, err = cli.GetUsersGoal()
	NoErr(t, err, "get user goal")

	if goal != expectedGoal {
		t.Errorf("got goal %q, expected %q", goal, expectedGoal)
	}
}

func TestAPIUserReviewees_APIUserReviewer(t *testing.T) {
	cli, teardown := setupInstance()
	defer teardown()

	// set up two teams. set up team mates on user's team. Set up a user from another team
	// set user from other team as a reviewer, make sure everyone shows up as reviewers.

	NoErr(t, cli.InsertTeam("team_1"), "creating team")
	NoErr(t, cli.InsertTeam("team_2"), "creating team")

	NoErr(t, CreateUser(cli.db, "user_1", "user_1@example.com"), "creating user")
	NoErr(t, CreateUser(cli.db, "user_2", "user_2@example.com"), "creating user")
	NoErr(t, CreateUser(cli.db, "user_3", "user_3@example.com"), "creating user")

	NoErr(t, cli.AssignTeamToUser("team_1"), "setting up user's team")
	NoErr(t, AssignTeamToUser(cli.db, "user_1@example.com", "team_1"), "setting up team")
	NoErr(t, AssignTeamToUser(cli.db, "user_2@example.com", "team_1"), "setting up team")
	NoErr(t, AssignTeamToUser(cli.db, "user_3@example.com", "team_2"), "setting up team")

	NoErr(t, cli.AddCycle("cycle_1"), "adding cycle")
	NoErr(t, cli.AddReviewer("user_3@example.com", "cycle_1"), "add reviewer")

	reviewees, err := cli.GetUserReviewees("cycle_1")
	NoErr(t, err, "getting reviewees")

	if len(reviewees) != 3 {
		t.Errorf("got %d reviewees, want %d - %v", len(reviewees), 4, reviewees)
	}
}

func TestAPIUserReviews(t *testing.T) {
	cli, teardown := setupInstance()
	defer teardown()

	NoErr(t, cli.AddCycle("cycle_1"), "adding cycle")

	err := cli.AddReviewForUser(cli.userEmail, "cycle_1", []string{"good at being awesome", "awesome at being good"}, []string{"follow through on tasks", "drive stories to completion"})
	NoErr(t, err, "adding review for user")

	reviews, err := cli.GetReviews()
	NoErr(t, err, "error getting reviews")

	if got, want := len(reviews), 1; got != want {
		t.Fatalf("got %d review(s), want %d", got, want)
	}

	if got, want := len(reviews[0].Opportunities), 2; got != want {
		t.Errorf("got %d opportunities, want %d", got, want)
	}

	if got, want := len(reviews[0].Strengths), 2; got != want {
		t.Errorf("got %d strengths, want %d", got, want)
	}
}

func NoErr(t *testing.T, err error, msg string) {
	_, fl, line, _ := runtime.Caller(1)
	path := strings.Split(fl, string(os.PathSeparator))
	file := path[len(path)-1]
	if err != nil {
		t.Errorf("[%s:%d] %v - %s", file, line, err, msg)
	}
}

// testClient embedds a *Client that will perform actions on behalf of a set auth key
// convenience properties such as the db and the logged in user's email are provided
type testClient struct {
	*Client
	db        *sql.DB
	userEmail string
}

func setupInstance() (*testClient, func() error) {
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

	return &testClient{cli, a.db, email}, func() error {
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
