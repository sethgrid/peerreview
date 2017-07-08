package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"
)

func TestAllTheThings(t *testing.T) {
	teardown := setupInstance()
	defer teardown()
	log.Println("in test")
}

func setupInstance() func() error {
	// todo: pass in optional seed
	log.Println("setting up test db")
	r := rand.New(rand.NewSource(time.Now().Unix()))
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

	go Serve(a, l)
	// log.Printf("test server starting on :%d", l.(net.TCPAddr).Port)

	return func() error {
		log.Println("tearing down db")
		if err := os.Remove(testDB); err != nil {
			log.Printf("unable to remove test db: %s - %v", testDB, err)
			return err
		}
		err = a.db.Close()
		if err != nil {
			log.Printf("unable to close db - %v", err)
			return err
		}
		return nil
	}
}
